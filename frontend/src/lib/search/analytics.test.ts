import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { API } from "$lib/api";
import {
	getSearchAttribution,
	recordSearchClick,
	recordSearchImpression,
	resolveSearchAttribution,
	revokePendingSearchConsent,
	searchConsent,
	setSearchConsent,
} from "./analytics";
let values: Map<string, string>;
beforeEach(() => {
	values = new Map();
	vi.stubGlobal("localStorage", {
		getItem: (key: string) => values.get(key) ?? null,
		setItem: (key: string, value: string) => values.set(key, value),
		removeItem: (key: string) => values.delete(key),
	});
});
afterEach(() => {
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
});
function apiStub() {
	return {
		createSearchProof: vi.fn().mockResolvedValue({ result_token: "proof" }),
		recordSearchImpression: vi.fn().mockResolvedValue({
			session_token: "session",
			impression_id: "impression",
		}),
		recordSearchEvent: vi.fn().mockResolvedValue({ event_id: "event", click_id: "click" }),
		revokeSearchConsent: vi.fn().mockResolvedValue(undefined),
	};
}
test("collection is opt-in and does not create a session before consent", async () => {
	const api = apiStub();
	expect(searchConsent()).toBe(false);
	await recordSearchImpression(api as unknown as API, "proof", "event");
	await recordSearchClick(api as unknown as API, { impressionId: "i", productId: 7, position: 1 });
	expect(api.recordSearchImpression).not.toHaveBeenCalled();
	expect(api.createSearchProof).not.toHaveBeenCalled();
	expect(api.recordSearchEvent).not.toHaveBeenCalled();
	expect(getSearchAttribution(7)).toBeUndefined();
});
test("impression retries preserve the supplied event identity and reuse the anonymous session", async () => {
	const api = apiStub();
	setSearchConsent(true);
	await recordSearchImpression(api as unknown as API, "proof", "same-event");
	await recordSearchImpression(api as unknown as API, "proof", "same-event");
	expect(api.recordSearchImpression).toHaveBeenLastCalledWith({
		consent: true,
		event_id: "same-event",
		session_token: "session",
		result_token: "proof",
	});
});
test("click attribution is product-specific and expires after seven days", async () => {
	const api = apiStub();
	setSearchConsent(true);
	await recordSearchImpression(api as unknown as API, "proof", "event");
	await recordSearchClick(api as unknown as API, {
		impressionId: "impression",
		productId: 7,
		position: 13,
	});
	expect(api.recordSearchEvent).toHaveBeenCalledWith(
		expect.objectContaining({
			session_token: "session",
			product_id: 7,
			position: 13,
			type: "click",
		})
	);
	expect(getSearchAttribution(7)).toEqual({ clickId: "click", sessionToken: "session" });
	expect(getSearchAttribution(8)).toBeUndefined();
	vi.spyOn(Date, "now").mockReturnValue(Date.now() + 7 * 24 * 60 * 60 * 1000);
	expect(getSearchAttribution(7)).toBeUndefined();
});
test("a failed newer click does not misattribute cart to a previous click", async () => {
	const api = apiStub();
	setSearchConsent(true);
	await recordSearchImpression(api as unknown as API, "proof", "event");
	await recordSearchClick(api as unknown as API, {
		impressionId: "impression",
		productId: 7,
		position: 1,
	});
	api.recordSearchEvent.mockRejectedValue(new Error("offline"));
	await expect(
		recordSearchClick(api as unknown as API, {
			impressionId: "impression",
			productId: 7,
			position: 2,
		})
	).rejects.toThrow("offline");
	expect(await resolveSearchAttribution(api as unknown as API, 7)).toBeUndefined();
});
test("opt-out clears local attribution immediately and retries failed server revocation", async () => {
	const api = apiStub();
	setSearchConsent(true);
	await recordSearchImpression(api as unknown as API, "proof", "event");
	await recordSearchClick(api as unknown as API, {
		impressionId: "impression",
		productId: 7,
		position: 1,
	});
	api.revokeSearchConsent.mockRejectedValueOnce(new Error("offline"));
	setSearchConsent(false, api as unknown as API);
	expect(getSearchAttribution(7)).toBeUndefined();
	expect(searchConsent()).toBe(false);
	await Promise.resolve();
	await revokePendingSearchConsent(api as unknown as API);
	expect(api.revokeSearchConsent).toHaveBeenCalledTimes(2);
	expect(values.get("search.analytics.pending-revocation")).toBe("[]");
});
test("an in-flight impression cannot restore a session after opt-out", async () => {
	let resolve!: (value: unknown) => void;
	const api = apiStub();
	api.recordSearchImpression.mockImplementation(
		() =>
			new Promise((r) => {
				resolve = r;
			})
	);
	setSearchConsent(true);
	const pending = recordSearchImpression(api as unknown as API, "proof", "event");
	await vi.waitFor(() => expect(api.recordSearchImpression).toHaveBeenCalled());
	setSearchConsent(false);
	resolve({ session_token: "session", impression_id: "impression" });
	expect(await pending).toBeUndefined();
	expect(values.has("search.analytics.session")).toBe(false);
});
test("invalid local attribution is ignored", () => {
	setSearchConsent(true);
	values.set("search.analytics.attribution", "not json");
	expect(getSearchAttribution(7)).toBeUndefined();
});
test("unavailable local storage leaves consent disabled", () => {
	vi.stubGlobal("localStorage", {
		getItem: () => {
			throw new Error("blocked");
		},
		setItem: () => {
			throw new Error("blocked");
		},
		removeItem: () => {
			throw new Error("blocked");
		},
	});
	expect(() => setSearchConsent(true)).not.toThrow();
	expect(searchConsent()).toBe(false);
	expect(getSearchAttribution(7)).toBeUndefined();
});
test("a lost first response retries with the same cryptographic session token", async () => {
	const api = apiStub();
	api.recordSearchImpression.mockRejectedValueOnce(new Error("response lost"));
	setSearchConsent(true);
	await expect(
		recordSearchImpression(api as unknown as API, "proof", "stable-event")
	).rejects.toThrow("response lost");
	const token = values.get("search.analytics.session");
	expect(token).toMatch(/^[0-9a-f]{64}$/);
	await recordSearchImpression(api as unknown as API, "proof", "stable-event");
	expect(api.recordSearchImpression.mock.calls[0][0].session_token).toBe(token);
	expect(api.recordSearchImpression.mock.calls[1][0].session_token).toBe(token);
});
test("a click is persisted before navigation and replayed idempotently before cart attribution", async () => {
	const api = apiStub();
	setSearchConsent(true);
	await recordSearchImpression(api as unknown as API, "proof", "impression-event");
	let finish!: (response: { event_id: string; click_id: string }) => void;
	api.recordSearchEvent.mockImplementationOnce(
		() =>
			new Promise((resolve) => {
				finish = resolve;
			})
	);
	const pending = recordSearchClick(api as unknown as API, {
		impressionId: "impression",
		productId: 7,
		position: 1,
	});
	expect(JSON.parse(values.get("search.analytics.pending-clicks")!)["7"].body).toMatchObject({
		product_id: 7,
		type: "click",
	});
	expect(await resolveSearchAttribution(api as unknown as API, 7)).toEqual({
		clickId: "click",
		sessionToken: "session",
	});
	expect(api.recordSearchEvent.mock.calls[1][0].event_id).toBe(
		api.recordSearchEvent.mock.calls[0][0].event_id
	);
	finish({ event_id: "event", click_id: "click" });
	await pending;
});
test("no proof means no impression and opt-out cancels pending click attribution", async () => {
	const api = apiStub();
	setSearchConsent(true);
	await recordSearchImpression(api as unknown as API, "", "event");
	expect(api.recordSearchImpression).not.toHaveBeenCalled();
	await recordSearchImpression(api as unknown as API, "proof", "event");
	api.recordSearchEvent.mockRejectedValueOnce(new Error("offline"));
	await expect(
		recordSearchClick(api as unknown as API, {
			impressionId: "impression",
			productId: 7,
			position: 1,
		})
	).rejects.toThrow();
	setSearchConsent(false);
	expect(await resolveSearchAttribution(api as unknown as API, 7)).toBeUndefined();
	expect(values.has("search.analytics.pending-clicks")).toBe(false);
});

test("consent withdrawal during proof exchange prevents collection and session creation", async () => {
	const api = apiStub();
	let resolve!: (value: { result_token: string }) => void;
	api.createSearchProof.mockImplementation(
		() =>
			new Promise((done) => {
				resolve = done;
			})
	);
	setSearchConsent(true);
	const pending = recordSearchImpression(api as unknown as API, "cached-snapshot", "event");
	setSearchConsent(false);
	resolve({ result_token: "proof" });
	await pending;
	expect(api.recordSearchImpression).not.toHaveBeenCalled();
	expect(values.has("search.analytics.session")).toBe(false);
});
