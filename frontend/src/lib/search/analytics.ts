import type { API } from "$lib/api";
import type { components } from "$lib/api/generated/openapi";
export const SEARCH_CONSENT_STORAGE_KEY = "search.analytics.consent";
const consentKey = SEARCH_CONSENT_STORAGE_KEY;
const sessionKey = "search.analytics.session";
const attributionKey = "search.analytics.attribution";
const pendingRevocationKey = "search.analytics.pending-revocation";
const attributionWindow = 7 * 24 * 60 * 60 * 1000;
export const SEARCH_CONSENT_CHANGED = "search:consent-changed";
type Attribution = { clickId: string; sessionToken: string; expiresAt: number };
let consentRevision = 0;
function read(key: string): string | null {
	try {
		return typeof localStorage === "undefined" ? null : localStorage.getItem(key);
	} catch {
		return null;
	}
}
function write(key: string, value?: string): void {
	try {
		if (typeof localStorage !== "undefined") {
			if (value === undefined) localStorage.removeItem(key);
			else localStorage.setItem(key, value);
		}
	} catch {
		/* Unavailable storage means collection remains disabled. */
	}
}
function pendingRevocations(): string[] {
	try {
		const value: unknown = JSON.parse(read(pendingRevocationKey) ?? "[]");
		return Array.isArray(value)
			? value.filter((token): token is string => typeof token === "string")
			: [];
	} catch {
		return [];
	}
}
function queueRevocation(token: string) {
	const pending = pendingRevocations();
	if (!pending.includes(token)) pending.push(token);
	write(pendingRevocationKey, JSON.stringify(pending));
}
export function searchConsent(): boolean {
	return read(consentKey) === "allowed";
}
export function setSearchConsent(enabled: boolean, api?: API): void {
	++consentRevision;
	write(consentKey, enabled ? "allowed" : "declined");
	if (!enabled) {
		const token = read(sessionKey);
		if (token) queueRevocation(token);
		write(sessionKey);
		write(attributionKey);
		if (api) void revokePendingSearchConsent(api);
	}
	if (typeof window !== "undefined") window.dispatchEvent(new Event(SEARCH_CONSENT_CHANGED));
}
export function getSearchAttribution(
	productId: number
): { clickId: string; sessionToken: string } | undefined {
	if (!searchConsent()) return;
	try {
		const parsed: unknown = JSON.parse(read(attributionKey) ?? "{}");
		if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return;
		const values = parsed as Record<string, Attribution>;
		const current = Object.fromEntries(
			Object.entries(values).filter(
				([, value]) =>
					value &&
					Number.isFinite(value.expiresAt) &&
					value.expiresAt > Date.now() &&
					typeof value.clickId === "string" &&
					value.clickId &&
					value.sessionToken === read(sessionKey)
			)
		);
		if (Object.keys(current).length !== Object.keys(values).length)
			write(attributionKey, JSON.stringify(current));
		const value = current[productId];
		return value ? { clickId: value.clickId, sessionToken: value.sessionToken } : undefined;
	} catch {
		return;
	}
}

export async function revokePendingSearchConsent(api: API) {
	for (const token of pendingRevocations()) {
		try {
			await api.revokeSearchConsent(token);
			write(
				pendingRevocationKey,
				JSON.stringify(pendingRevocations().filter((item) => item !== token))
			);
		} catch {
			/* Retain for retry on the next visit. */
		}
	}
}
export async function recordSearchImpression(
	api: API,
	filters: components["schemas"]["SearchPreviewFilters"],
	eventId: string
) {
	if (!searchConsent()) return;
	const revision = consentRevision;
	let sessionToken = read(sessionKey);
	if (!sessionToken) {
		try {
			sessionToken = Array.from(crypto.getRandomValues(new Uint8Array(32)), (byte) =>
				byte.toString(16).padStart(2, "0")
			).join("");
		} catch {
			return;
		}
		write(sessionKey, sessionToken);
		if (read(sessionKey) !== sessionToken) return;
	}
	const response = await api.recordSearchImpression({
		consent: true,
		event_id: eventId,
		session_token: sessionToken,
		filters,
	});
	if (!searchConsent() || consentRevision !== revision) {
		queueRevocation(response.session_token);
		void revokePendingSearchConsent(api);
		return;
	}
	write(sessionKey, response.session_token);
	return response;
}
export async function recordSearchClick(
	api: API,
	input: { impressionId: string; productId: number; position: number }
) {
	if (!searchConsent()) return;
	const revision = consentRevision;
	const sessionToken = read(sessionKey);
	if (!sessionToken) return;
	const response = await api.recordSearchEvent({
		session_token: sessionToken,
		impression_id: input.impressionId,
		event_id: crypto.randomUUID(),
		type: "click",
		product_id: input.productId,
		position: input.position,
	});
	if (
		!searchConsent() ||
		consentRevision !== revision ||
		!response.click_id ||
		read(sessionKey) !== sessionToken
	)
		return;
	let values: Record<string, Attribution> = {};
	try {
		const parsed = JSON.parse(read(attributionKey) ?? "{}");
		if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) values = parsed;
	} catch {
		/* Ignore invalid local state. */
	}
	values[input.productId] = {
		clickId: response.click_id,
		sessionToken,
		expiresAt: Date.now() + attributionWindow,
	};
	write(attributionKey, JSON.stringify(values));
	return response.click_id;
}
