// Field reading and validation.
//
// Separate from index.js for the same reason as mime.js: these are the rules
// that decide what a stranger can put in our database, and they should be
// assertable without the Workers runtime.

export const LIMITS = {
  message: 5000,
  email: 254,
  name: 120,
  org_name: 200,
  page_url: 500,
};

// Mirrors the <select> on /report. An unknown value becomes "other" rather
// than a rejection: a stale cached form should not lose a real report.
export const PROBLEMS = new Set([
  "out-of-date",
  "wrong",
  "org-gone",
  "org-details",
  "broken-link",
  "other",
]);

export const MIN_MESSAGE = 10;

export function field(form, key) {
  const v = form.get(key);
  return typeof v === "string" ? v.trim() : "";
}

export function clamp(s, max) {
  return s.length > max ? s.slice(0, max) : s;
}

export function problemOf(v) {
  return PROBLEMS.has(v) ? v : "other";
}

export function validEmail(s) {
  return /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(s);
}

/**
 * Keeps only a path on our own site.
 *
 * page_url arrives in a hidden field, so treat it as attacker-controlled. It
 * ends up in a notification email we click, and in a column we group reports
 * by. Anything pointing elsewhere is dropped rather than rejected, because the
 * reader's actual report is still worth having.
 */
export function sameSiteURL(raw, siteOrigin) {
  if (!raw) return "";
  try {
    const u = new URL(raw, siteOrigin);
    if (u.origin !== siteOrigin) return "";
    return clamp(u.pathname + u.search, LIMITS.page_url);
  } catch {
    return "";
  }
}
