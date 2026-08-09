import { describe, expect, it } from "vitest";
import { ApiProblemError, getApiErrorMessage, isApiProblem, isApiProblemError } from "./errors";

describe("ApiProblemError", () => {
	it("keeps the legacy status, statusText, and body fields", () => {
		const body = { error: "Email is already registered", code: "duplicate_email" };
		const error = new ApiProblemError(409, "Conflict", body);

		expect(error).toBeInstanceOf(Error);
		expect(error.status).toBe(409);
		expect(error.statusText).toBe("Conflict");
		expect(error.body).toBe(body);
		expect(error.message).toBe("Email is already registered");
		expect(isApiProblemError(error)).toBe(true);
		expect(isApiProblem(error)).toBe(true);
	});

	it("recognizes legacy structural API problems without treating them as Error instances", () => {
		const legacy = { status: 401, statusText: "Unauthorized", body: "sign in required" };

		expect(isApiProblem(legacy)).toBe(true);
		expect(isApiProblemError(legacy)).toBe(false);
		expect(getApiErrorMessage(legacy, "fallback")).toBe("sign in required");
	});
});

describe("getApiErrorMessage", () => {
	it("extracts common API problem fields in priority order", () => {
		expect(getApiErrorMessage({ body: { error: "specific", detail: "detail" } })).toBe("specific");
		expect(getApiErrorMessage({ detail: "RFC problem detail" })).toBe("RFC problem detail");
		expect(getApiErrorMessage(new Error("network failed"))).toBe("network failed");
	});

	it("uses the fallback for malformed or empty payloads", () => {
		expect(getApiErrorMessage({ body: { error: 42 } }, "Unable to continue.")).toBe(
			"Unable to continue."
		);
		expect(getApiErrorMessage(null, "Unable to continue.")).toBe("Unable to continue.");
	});
});
