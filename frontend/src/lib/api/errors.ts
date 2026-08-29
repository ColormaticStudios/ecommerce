export interface ApiProblem {
	status: number;
	statusText: string;
	body: unknown;
}

export interface LocalizedProblemBody {
	error_code: string;
	message_key?: string;
	message_params?: Record<string, string | number | boolean>;
	detail?: string;
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
	readonly errorCode: string;
	readonly messageKey: string;
	readonly messageParams: Record<string, string | number | boolean>;

	constructor(status: number, statusText: string, body: unknown) {
		super(extractMessage(body) || statusText || `API request failed with status ${status}`);
		this.name = "ApiProblemError";
		this.status = status;
		this.statusText = statusText;
		this.body = body;
		this.errorCode = isRecord(body) && typeof body.error_code === "string" ? body.error_code : "";
		this.messageKey =
			isRecord(body) && typeof body.message_key === "string" ? body.message_key : "";
		this.messageParams = {};
		if (isRecord(body) && isRecord(body.message_params)) {
			for (const [key, value] of Object.entries(body.message_params)) {
				if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
					this.messageParams[key] = value;
				}
			}
		}
	}
}

export function getLocalizedApiErrorMessage(
	error: unknown,
	translate: (
		key: string,
		source: string,
		parameters?: Record<string, string | number | boolean>
	) => string,
	fallback = ""
): string {
	if (error instanceof ApiProblemError && error.messageKey) {
		return translate(error.messageKey, getApiErrorMessage(error, fallback), error.messageParams);
	}
	return getApiErrorMessage(error, fallback);
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
