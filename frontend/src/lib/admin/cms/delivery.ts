import type { components } from "$lib/api/generated/openapi";

export type CmsPublication = components["schemas"]["CmsPublication"];
export type CmsEntryVersion = components["schemas"]["CmsEntryVersion"];

export interface DeliveryRuleDraft {
	id: string;
	enabled: boolean;
	markets: string;
	devices: Array<"desktop" | "mobile" | "tablet">;
	audience: "all" | "guest" | "authenticated";
	referrers: string;
	utmSources: string;
	segments: string;
}

export function splitDeliveryValues(value: string): string[] {
	return value
		.split(",")
		.map((item) => item.trim())
		.filter(Boolean);
}

export function toggleDeliveryRuleDevice(
	rule: DeliveryRuleDraft,
	device: DeliveryRuleDraft["devices"][number]
): DeliveryRuleDraft {
	return {
		...rule,
		devices: rule.devices.includes(device)
			? rule.devices.filter((item) => item !== device)
			: [...rule.devices, device],
	};
}
