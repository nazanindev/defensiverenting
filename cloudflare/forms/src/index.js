// Form intake for renterlaw.org.
//
// The public site is a Go server on Fly with a DNS-only record, so this Worker
// answers on its own hostname and the form posts to it cross-origin. A plain
// HTML form POST is a top-level navigation: no preflight, no CORS headers, and
// no dependency on fetch() succeeding. The Worker replies with a 303 back to
// the site, so the reader ends up on renterlaw.org either way.
//
// Order matters below. Cheap rejections (honeypot, field shape) run before
// Turnstile, and Turnstile runs before any database write, so the common bot
// submission costs one subrequest at most and never touches D1.

import { EmailMessage } from "cloudflare:email";
import { mime } from "./mime.js";
import {
  LIMITS,
  MIN_MESSAGE,
  clamp,
  field,
  problemOf,
  sameSiteURL,
  validEmail,
} from "./fields.js";

const MAX_PER_HOUR = 5;

export default {
  async fetch(request, env) {
    const url = new URL(request.url);

    const kind =
      url.pathname === "/report" ? "report" :
      url.pathname === "/contact" ? "contact" :
      null;

    if (!kind) return new Response("Not found\n", { status: 404 });

    if (request.method !== "POST") {
      // Someone opened the endpoint directly. Send them to the real form.
      return redirect(`${env.SITE_ORIGIN}/${kind}`);
    }

    // Checked only when present. Some privacy tools strip Origin, and Turnstile
    // already binds a submission to our site key, so a missing header is not
    // worth losing a report over.
    const origin = request.headers.get("Origin");
    if (origin && origin !== env.SITE_ORIGIN) {
      return fail(env, kind, "origin");
    }

    let form;
    try {
      form = await request.formData();
    } catch {
      return fail(env, kind, "badrequest");
    }

    // Honeypot. Real browsers leave it empty because it is hidden; bots that
    // fill every field land here. Answer with the success page so the bot has
    // no signal to retry with the field cleared.
    if (field(form, "company")) return thanks(env, kind);

    const message = clamp(field(form, "message"), LIMITS.message);
    if (message.length < MIN_MESSAGE) return fail(env, kind, "message");

    const email = clamp(field(form, "email"), LIMITS.email);
    if (email && !validEmail(email)) return fail(env, kind, "email");

    const token = field(form, "cf-turnstile-response");
    const ip = request.headers.get("CF-Connecting-IP") || "";
    if (!(await verifyTurnstile(env, token, ip))) {
      return fail(env, kind, "captcha");
    }

    const isReport = kind === "report";
    const submission = {
      id: crypto.randomUUID(),
      kind,
      name: clamp(field(form, "name"), LIMITS.name),
      email,
      message,
      page_url: isReport ? sameSiteURL(field(form, "page_url"), env.SITE_ORIGIN) : "",
      org_name: isReport ? clamp(field(form, "org_name"), LIMITS.org_name) : "",
      problem: isReport ? problemOf(field(form, "problem")) : "",
      country: request.headers.get("CF-IPCountry") || "",
      user_agent: clamp(request.headers.get("User-Agent") || "", 300),
      ip_hash: await sha256(`${env.IP_SALT}|${ip}`),
    };

    try {
      if (await overLimit(env, submission.ip_hash)) {
        return fail(env, kind, "ratelimit");
      }
      await store(env, submission);
    } catch (err) {
      console.error("d1 write failed", err);
      return fail(env, kind, "server");
    }

    // The submission is already durable. A mail failure is worth logging and
    // nothing more: making the reader retype a report because SMTP hiccuped
    // would lose the report and duplicate the row.
    try {
      await notify(env, submission);
    } catch (err) {
      console.error("notify failed", err);
    }

    return thanks(env, kind);
  },
};

// ── Spam check ──────────────────────────────────────────────

async function verifyTurnstile(env, token, ip) {
  if (!token) return false;
  const body = new FormData();
  body.append("secret", env.TURNSTILE_SECRET_KEY);
  body.append("response", token);
  if (ip) body.append("remoteip", ip);

  try {
    const res = await fetch(
      "https://challenges.cloudflare.com/turnstile/v0/siteverify",
      { method: "POST", body },
    );
    const data = await res.json();
    if (!data.success) console.warn("turnstile rejected", data["error-codes"]);
    return data.success === true;
  } catch (err) {
    // Fail closed. An unreachable spam check is not a reason to accept
    // everything, and the reader gets a retry rather than silence.
    console.error("turnstile unreachable", err);
    return false;
  }
}

async function sha256(s) {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(s));
  return [...new Uint8Array(digest)]
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

// ── Storage ─────────────────────────────────────────────────

async function overLimit(env, ipHash) {
  const row = await env.DB.prepare(
    `SELECT COUNT(*) AS n FROM submissions
      WHERE ip_hash = ? AND created_at > datetime('now', '-1 hour')`,
  )
    .bind(ipHash)
    .first();
  return (row?.n ?? 0) >= MAX_PER_HOUR;
}

async function store(env, s) {
  await env.DB.prepare(
    `INSERT INTO submissions
       (id, kind, created_at, page_url, org_name, problem,
        name, email, message, ip_hash, country, user_agent)
     VALUES (?, ?, datetime('now'), ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
  )
    .bind(
      s.id, s.kind, s.page_url || null, s.org_name || null, s.problem || null,
      s.name || null, s.email || null, s.message, s.ip_hash,
      s.country || null, s.user_agent || null,
    )
    .run();
}

// ── Notification mail ───────────────────────────────────────

const PROBLEM_LABELS = {
  "out-of-date": "Information is out of date",
  "wrong": "Information is wrong",
  "org-gone": "Organization no longer exists",
  "org-details": "Organization details changed",
  "broken-link": "Link is broken",
  "other": "Something else",
};

async function notify(env, s) {
  const subject =
    s.kind === "report"
      ? `[RenterLaw] ${PROBLEM_LABELS[s.problem] || "Report"}: ${s.page_url || "site"}`
      : `[RenterLaw] Contact from ${s.name || s.email || "a reader"}`;

  const lines = [];
  if (s.page_url) lines.push(`Page:    ${env.SITE_ORIGIN}${s.page_url}`);
  if (s.org_name) lines.push(`Org:     ${s.org_name}`);
  if (s.problem) lines.push(`Problem: ${PROBLEM_LABELS[s.problem] || s.problem}`);
  if (s.name) lines.push(`Name:    ${s.name}`);
  lines.push(`Reply:   ${s.email || "(no address given)"}`);
  lines.push(`Country: ${s.country || "unknown"}`);
  lines.push(`ID:      ${s.id}`);
  lines.push("", s.message, "");
  lines.push(`Mark handled:
  wrangler d1 execute renterlaw-forms --remote \\
    --command "UPDATE submissions SET status='resolved' WHERE id='${s.id}'"`);

  const raw = mime({
    from: `RenterLaw Forms <${env.MAIL_FROM}>`,
    to: env.MAIL_TO,
    // Lets a reply go straight to the reader. The address passed validEmail,
    // so it holds no whitespace and cannot break out of the header.
    replyTo: s.email || "",
    messageId: `<${s.id}@${env.MAIL_FROM.split("@")[1]}>`,
    subject,
    body: lines.join("\n"),
  });

  await env.NOTIFY.send(new EmailMessage(env.MAIL_FROM, env.MAIL_TO, raw));
}

// ── Responses ───────────────────────────────────────────────

// 303 so the browser turns the POST into a GET. Reloading the thank-you page
// then costs nothing instead of resubmitting the form.
function redirect(location) {
  return new Response(null, { status: 303, headers: { Location: location } });
}

function thanks(env, kind) {
  return redirect(`${env.SITE_ORIGIN}/thanks?k=${encodeURIComponent(kind)}`);
}

function fail(env, kind, code) {
  return redirect(`${env.SITE_ORIGIN}/${kind}?error=${encodeURIComponent(code)}`);
}
