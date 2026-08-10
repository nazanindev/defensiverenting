// Run with: node --test
//
// Covers the two pieces of the Worker that fail quietly: the URL filter that
// decides what a stranger can put in our database, and the message builder
// whose bugs show up as notifications not arriving.

import { strict as assert } from "node:assert";
import { test, describe } from "node:test";

import { clamp, problemOf, sameSiteURL, validEmail } from "../src/fields.js";
import { base64, encodeWord, headerSafe, mime, wrap76 } from "../src/mime.js";

const SITE = "https://renterlaw.org";

describe("sameSiteURL", () => {
  test("keeps paths on our own site", () => {
    assert.equal(sameSiteURL("/j/boston/repairs", SITE), "/j/boston/repairs");
    assert.equal(sameSiteURL("/t/repairs?j=boston", SITE), "/t/repairs?j=boston");
    assert.equal(sameSiteURL(`${SITE}/about`, SITE), "/about");
  });

  test("drops anything pointing elsewhere", () => {
    for (const raw of [
      "https://evil.example/pwn",
      "//evil.example/pwn",
      "javascript:alert(1)",
      "data:text/html,<script>alert(1)</script>",
      "http://renterlaw.org.evil.example/",
      // Right host, wrong scheme: the origin comparison catches it.
      "http://renterlaw.org/about",
    ]) {
      assert.equal(sameSiteURL(raw, SITE), "", `should have dropped ${raw}`);
    }
  });

  test("handles empty and unparseable input", () => {
    assert.equal(sameSiteURL("", SITE), "");
    assert.equal(sameSiteURL("http://[", SITE), "");
  });

  test("truncates rather than rejecting a very long path", () => {
    const got = sameSiteURL("/" + "a".repeat(9000), SITE);
    assert.equal(got.length, 500);
    assert.ok(got.startsWith("/aaa"));
  });
});

describe("field validation", () => {
  test("an unknown problem becomes other, not a rejection", () => {
    assert.equal(problemOf("org-gone"), "org-gone");
    assert.equal(problemOf("from-a-stale-cached-form"), "other");
    assert.equal(problemOf(""), "other");
  });

  test("email shape", () => {
    assert.ok(validEmail("someone@example.org"));
    assert.ok(!validEmail("someone@example"));
    assert.ok(!validEmail("two addresses@a.com b@c.com"));
    assert.ok(!validEmail("has\nnewline@a.com"));
  });

  test("clamp leaves short values alone", () => {
    assert.equal(clamp("short", 100), "short");
    assert.equal(clamp("abcdef", 3), "abc");
  });
});

describe("mime", () => {
  const base = {
    from: "RenterLaw Forms <forms@renterlaw.org>",
    to: "inbox@example.org",
    messageId: "<abc-123@renterlaw.org>",
    subject: "[RenterLaw] Report",
    body: "The office on Main St closed last month.",
    date: "Mon, 10 Aug 2026 12:00:00 +0000",
  };

  test("headers are CRLF separated and end with a blank line", () => {
    const msg = mime(base);
    const [headers, body] = msg.split("\r\n\r\n");
    assert.ok(headers.includes("From: RenterLaw Forms <forms@renterlaw.org>"));
    assert.ok(headers.includes("To: inbox@example.org"));
    assert.ok(headers.includes("Subject: [RenterLaw] Report"));
    assert.ok(headers.includes("MIME-Version: 1.0"));
    assert.ok(!headers.includes("\n\n"), "headers must use CRLF only");
    assert.equal(Buffer.from(body.trim(), "base64").toString("utf8"), base.body);
  });

  test("Reply-To is omitted when the reader left no address", () => {
    assert.ok(!mime(base).includes("Reply-To:"));
    assert.ok(mime({ ...base, replyTo: "r@example.org" }).includes("Reply-To: r@example.org"));
  });

  // The reader supplies the Reply-To address. A newline in it would end the
  // header and start one of the sender's choosing.
  test("header injection through a field is neutralised", () => {
    const msg = mime({ ...base, replyTo: "a@b.com\r\nBcc: victim@example.org" });

    // The check is that no header LINE begins with Bcc. The text still appears,
    // folded into the Reply-To value, which is the point: it stayed data.
    const headerLines = msg.split("\r\n\r\n")[0].split("\r\n");
    assert.ok(
      !headerLines.some((l) => l.toLowerCase().startsWith("bcc:")),
      "Bcc became its own header",
    );
    assert.ok(headerLines.includes("Reply-To: a@b.com Bcc: victim@example.org"));
  });

  test("a non-ASCII subject is RFC 2047 encoded", () => {
    const msg = mime({ ...base, subject: "Informe sobre alquiler en español" });
    const line = msg.split("\r\n").find((l) => l.startsWith("Subject:"));
    assert.ok(line.includes("=?UTF-8?B?"), `not encoded: ${line}`);
    const encoded = line.slice("Subject: =?UTF-8?B?".length, -2);
    assert.equal(Buffer.from(encoded, "base64").toString("utf8"), "Informe sobre alquiler en español");
  });

  test("a non-ASCII body survives the round trip", () => {
    const body = "Le bureau a fermé. 房东没有退还押金。🏠";
    const msg = mime({ ...base, body });
    const encoded = msg.split("\r\n\r\n")[1].replaceAll("\r\n", "");
    assert.equal(Buffer.from(encoded, "base64").toString("utf8"), body);
  });

  test("base64 body lines stay within 76 columns", () => {
    const msg = mime({ ...base, body: "x".repeat(5000) });
    for (const line of msg.split("\r\n\r\n")[1].split("\r\n")) {
      assert.ok(line.length <= 76, `line of ${line.length} columns`);
    }
  });

  test("a body at the field limit does not blow the stack", () => {
    const body = "é".repeat(5000);
    assert.equal(
      Buffer.from(wrap76(base64(body)).replaceAll("\r\n", ""), "base64").toString("utf8"),
      body,
    );
  });
});

describe("mime helpers", () => {
  test("headerSafe collapses CR and LF", () => {
    assert.equal(headerSafe("a\r\nb\nc"), "a b c");
    assert.equal(headerSafe("  padded  "), "padded");
  });

  test("encodeWord leaves plain ASCII alone", () => {
    assert.equal(encodeWord("[RenterLaw] Report: /j/boston"), "[RenterLaw] Report: /j/boston");
  });
});
