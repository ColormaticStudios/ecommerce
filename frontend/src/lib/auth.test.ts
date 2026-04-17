import { expect, test } from "vitest";
import { buildOIDCLoginUrl, sanitizeAuthRedirectPath } from "./auth";

test("sanitizeAuthRedirectPath keeps internal paths and drops external targets", () => {
	expect(sanitizeAuthRedirectPath("/checkout?step=payment#saved")).toBe(
		"/checkout?step=payment#saved"
	);
	expect(sanitizeAuthRedirectPath("https://attacker.example/phish")).toBe("/");
	expect(sanitizeAuthRedirectPath("javascript:alert(1)")).toBe("/");
	expect(sanitizeAuthRedirectPath(undefined)).toBe("/");
});

test("buildOIDCLoginUrl encodes a sanitized redirect path", () => {
	expect(buildOIDCLoginUrl("https://api.example.com/", "/orders?filter=open#recent")).toBe(
		"https://api.example.com/api/v1/auth/oidc/login?redirect=%2Forders%3Ffilter%3Dopen%23recent"
	);
	expect(buildOIDCLoginUrl("https://api.example.com", "https://attacker.example")).toBe(
		"https://api.example.com/api/v1/auth/oidc/login?redirect=%2F"
	);
});
