const blockPathPattern = /payload\.blocks\[(\d+)\]((?:\.[a-z_]+|\[[^\]]+\])*)\s+(.+)$/i;

const fieldLabels: Record<string, string> = {
	body: "body text",
	media_id: "image",
	images: "image",
	url: "link URL",
	label: "link label",
	title: "title",
	items: "FAQ item",
	question: "question",
	answer: "answer",
	html: "HTML content",
	type: "section type",
	source: "product source",
	category_slug: "category",
	limit: "product limit",
	sort: "sorting option",
	order: "sort order",
	category_slugs: "category",
	category_media_ids: "category image",
	image_aspect: "image shape",
	campaign_id: "campaign",
	product_id: "product",
	low_stock_threshold: "low-stock threshold",
	quote: "quote",
	attribution: "attribution",
	rating: "rating",
	provider: "social provider",
	link: "link",
	primary_cta: "primary action",
};

function rawErrorMessage(error: unknown): string {
	if (typeof error === "string") return error;
	if (!error || typeof error !== "object") return "";
	const candidate = error as { body?: { error?: unknown }; message?: unknown };
	if (typeof candidate.body?.error === "string") return candidate.body.error;
	return typeof candidate.message === "string" ? candidate.message : "";
}

function pathDetails(path: string) {
	const segments = path.match(/[a-z_]+|\[[^\]]+\]/gi) ?? [];
	const names = segments.filter((segment) => !segment.startsWith("[")).map((segment) => segment);
	const nestedIndex = segments.find((segment) => /^\[\d+\]$/.test(segment));
	const container = names[0] ?? "";
	const field = names.at(-1) ?? "section";
	let label = fieldLabels[field] ?? field.replaceAll("_", " ");

	if (container === "link" && field === "label") label = "link label";
	if (container === "link" && field === "url") label = "link URL";
	if (container === "primary_cta" && field === "label") label = "primary action label";
	if (container === "primary_cta" && field === "url") label = "primary action URL";

	return {
		label,
		nestedLabel:
			nestedIndex && container === "images"
				? `, image #${Number(nestedIndex.slice(1, -1)) + 1}`
				: nestedIndex && container === "items"
					? `, FAQ item #${Number(nestedIndex.slice(1, -1)) + 1}`
					: "",
	};
}

function formatBlockError(raw: string): string | null {
	const match = raw.match(blockPathPattern);
	if (!match) return null;
	const blockNumber = Number(match[1]) + 1;
	const { label, nestedLabel } = pathDetails(match[2] ?? "");
	const reason = (match[3] ?? "").replace(/[.]$/, "");
	const subject = `Block #${blockNumber}${nestedLabel}`;

	if (reason === "is required" || reason === "must be a non-empty array") {
		return `${subject} is missing ${label}.`;
	}
	const range = reason.match(/^must be between (.+)$/i);
	if (range) return `${subject} has an invalid ${label}. It must be between ${range[1]}.`;
	if (reason === "must be positive") return `${subject} needs a valid ${label}.`;
	if (reason === "is unsafe" || reason.startsWith("is not allowed")) {
		return `${subject} has a ${label} that isn't allowed.`;
	}
	if (reason === "is unsupported") return `${subject} has an invalid ${label}.`;
	if (reason === "must be an object" || reason === "must be an array" || reason === "is invalid") {
		return `${subject} has invalid ${label} settings.`;
	}
	return `${subject} needs attention: ${label} ${reason}.`;
}

export function formatCmsAdminError(error: unknown, fallback: string): string {
	const raw = rawErrorMessage(error).trim();
	if (!raw) return fallback;
	const blockError = formatBlockError(raw);
	if (blockError) return blockError;
	if (/unsupported block type/i.test(raw)) {
		return "This page contains a section type that is no longer supported.";
	}

	const cleaned = raw.replace(/^invalid cms (?:page|delivery configuration):\s*/i, "").trim();
	if (!cleaned || (cleaned === raw && /payload\.|\[[0-9]+\]/.test(cleaned))) return fallback;
	return `${cleaned.charAt(0).toUpperCase()}${cleaned.slice(1).replace(/[.]$/, "")}.`;
}
