import type { components } from "$lib/api/generated/openapi";

type CheckoutProvider = components["schemas"]["CheckoutPlugin"];
type CheckoutProviderField = components["schemas"]["CheckoutPluginField"];
type CheckoutProviderState = components["schemas"]["CheckoutPluginState"];

export function initDataForProvider(
	fields: CheckoutProviderField[] | undefined,
	dataMap: Record<string, string>
) {
	for (const field of fields ?? []) {
		if (dataMap[field.key] !== undefined) {
			continue;
		}
		if (field.type === "checkbox") {
			dataMap[field.key] = "false";
			continue;
		}
		if (field.type === "select") {
			dataMap[field.key] = field.options?.[0]?.value ?? "";
			continue;
		}
		dataMap[field.key] = "";
	}
}

export function providerUsesCardFields(provider: CheckoutProvider | null): boolean {
	if (!provider) {
		return false;
	}
	const keys = new Set((provider.fields ?? []).map((field) => field.key));
	return keys.has("card_number") || (keys.has("exp_month") && keys.has("exp_year"));
}

export function providerUsesAddressFields(provider: CheckoutProvider | null): boolean {
	if (!provider) {
		return false;
	}
	const keys = new Set((provider.fields ?? []).map((field) => field.key));
	return keys.has("line1") && keys.has("city") && keys.has("postal_code") && keys.has("country");
}

export function providerLogoMark(provider: CheckoutProvider): string {
	const words = provider.name
		.split(/\s+/)
		.map((value) => value.trim())
		.filter(Boolean);
	if (words.length === 0) {
		return "PV";
	}
	if (words.length === 1) {
		return words[0].slice(0, 2).toUpperCase();
	}
	return `${words[0][0] ?? "P"}${words[1][0] ?? "V"}`.toUpperCase();
}

export function providerLogoColor(providerID: string): string {
	switch (providerID) {
		case "dummy-card":
			return "bg-emerald-100 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-300";
		case "dummy-wallet":
			return "bg-sky-100 text-sky-700 dark:bg-sky-950/60 dark:text-sky-300";
		case "dummy-ground":
			return "bg-orange-100 text-orange-700 dark:bg-orange-950/60 dark:text-orange-300";
		case "dummy-pickup":
			return "bg-indigo-100 text-indigo-700 dark:bg-indigo-950/60 dark:text-indigo-300";
		default:
			return "bg-gray-200 text-gray-700 dark:bg-gray-800 dark:text-gray-200";
	}
}

export function maskedCardDisplay(last4: string | undefined): string {
	const digits = (last4 ?? "").padStart(4, "0").slice(-4);
	return `•••• •••• •••• ${digits}`;
}

export function stateTone(severity: CheckoutProviderState["severity"]) {
	switch (severity) {
		case "error":
			return "border-red-200 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300";
		case "warning":
			return "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300";
		case "success":
			return "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300";
		default:
			return "border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900 dark:bg-blue-950/30 dark:text-blue-300";
	}
}
