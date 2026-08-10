// RFC 5322 message building.
//
// Separate from index.js so it can be tested under plain node: index.js imports
// cloudflare:email, which only resolves inside the Workers runtime. A malformed
// message is the kind of bug that shows up as notifications quietly not
// arriving, so it is worth being able to assert on the bytes.

/**
 * Builds a plain-text message. The body is base64 UTF-8 rather than raw text
 * because submissions carry names, addresses, and accented characters that
 * 7-bit ASCII would mangle.
 */
export function mime({ from, to, replyTo, messageId, subject, body, date }) {
  const headers = [
    `From: ${headerSafe(from)}`,
    `To: ${headerSafe(to)}`,
    replyTo ? `Reply-To: ${headerSafe(replyTo)}` : null,
    `Message-ID: ${headerSafe(messageId)}`,
    `Date: ${date || rfc5322Date()}`,
    `Subject: ${encodeWord(headerSafe(subject))}`,
    "MIME-Version: 1.0",
    'Content-Type: text/plain; charset="utf-8"',
    "Content-Transfer-Encoding: base64",
  ].filter(Boolean);

  return headers.join("\r\n") + "\r\n\r\n" + wrap76(base64(body)) + "\r\n";
}

// A newline in a header value would let user input inject headers of its own.
// Reply-To is built from a reader-supplied address, so this is load-bearing.
export function headerSafe(s) {
  return String(s).replace(/[\r\n]+/g, " ").trim();
}

// Non-ASCII headers need RFC 2047 encoding or they arrive as mojibake.
export function encodeWord(s) {
  // eslint-disable-next-line no-control-regex
  return /^[\x20-\x7E]*$/.test(s) ? s : `=?UTF-8?B?${base64(s)}?=`;
}

export function rfc5322Date(now = new Date()) {
  // toUTCString ends in "GMT"; the spec wants a numeric offset.
  return now.toUTCString().replace(/GMT$/, "+0000");
}

export function base64(str) {
  const bytes = new TextEncoder().encode(str);
  let binary = "";
  const CHUNK = 0x8000; // apply() blows the stack somewhere above this
  for (let i = 0; i < bytes.length; i += CHUNK) {
    binary += String.fromCharCode(...bytes.subarray(i, i + CHUNK));
  }
  return btoa(binary);
}

// Base64 bodies are wrapped at 76 columns. Some strict parsers reject longer
// lines outright.
export function wrap76(s) {
  return (s.match(/.{1,76}/g) || []).join("\r\n");
}
