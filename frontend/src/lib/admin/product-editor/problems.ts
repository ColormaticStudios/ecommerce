import { getApiErrorMessage } from "$lib/api/errors";

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function bodyError(error: unknown): string {
	if (!isRecord(error) || !isRecord(error.body)) return "";
	const message = error.body.error;
	return typeof message === "string" ? message.trim() : "";
}

export function mapProductEditorProblem(error: unknown, fallback: string): string {
	return bodyError(error) || fallback;
}

export function mapMediaUploadProblem(error: unknown): string {
	if (isRecord(error) && error.status === 409) {
		return bodyError(error) || getApiErrorMessage(error, "Unable to upload media.");
	}
	return "Unable to upload media.";
}
