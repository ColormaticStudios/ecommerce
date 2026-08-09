export interface ApiProblem {
	status: number;
	statusText: string;
	body: unknown;
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function extractMessage(value: unknown): string {
	if (typeof value === "string") {
		return value.trim();
	}
	if (!isRecord(value)) {
		return "";
	}

	for (const key of ["error", "detail", "message", "title"] as const) {
		const candidate = value[key];
		if (typeof candidate === "string" && candidate.trim()) {
			return candidate.trim();
		}
	}
	return "";
}

export function isApiProblem(value: unknown): value is ApiProblem {
	return (
		isRecord(value) &&
		typeof value.status === "number" &&
		typeof value.statusText === "string" &&
		"body" in value
	);
}

export class ApiProblemError extends Error implements ApiProblem {
	readonly status: number;
	readonly statusText: string;
	readonly body: unknown;

	constructor(status: number, statusText: string, body: unknown) {
		super(extractMessage(body) || statusText || `API request failed with status ${status}`);
		this.name = "ApiProblemError";
		this.status = status;
		this.statusText = statusText;
		this.body = body;
	}
}

export function isApiProblemError(value: unknown): value is ApiProblemError {
	return value instanceof ApiProblemError;
}

export function getApiErrorMessage(error: unknown, fallback = ""): string {
	if (isRecord(error) && "body" in error) {
		const bodyMessage = extractMessage(error.body);
		if (bodyMessage) {
			return bodyMessage;
		}
	}
	if (error instanceof Error) {
		return error.message.trim() || fallback;
	}
	return extractMessage(error) || fallback;
}
