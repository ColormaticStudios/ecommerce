export interface PublicRuntimeEnv {
	PUBLIC_API_BASE_URL?: string;
	STORYBOOK_PUBLIC_API_BASE_URL?: string;
}

const DEFAULT_PUBLIC_API_BASE_URL = "http://localhost:3000";

export function readConfiguredPublicApiBaseUrl(env?: PublicRuntimeEnv | null): string | undefined {
	if (!env) {
		return undefined;
	}

	return env.PUBLIC_API_BASE_URL || env.STORYBOOK_PUBLIC_API_BASE_URL || undefined;
}

export function resolvePublicApiBaseUrl({
	serverEnv,
	clientEnv,
	fallback = DEFAULT_PUBLIC_API_BASE_URL,
}: {
	serverEnv?: PublicRuntimeEnv | null;
	clientEnv?: PublicRuntimeEnv | null;
	fallback?: string;
} = {}): string {
	return (
		readConfiguredPublicApiBaseUrl(serverEnv) ||
		readConfiguredPublicApiBaseUrl(clientEnv) ||
		fallback
	);
}

export function readServerPublicRuntimeEnv(): PublicRuntimeEnv {
	return {
		PUBLIC_API_BASE_URL: process.env.PUBLIC_API_BASE_URL,
		STORYBOOK_PUBLIC_API_BASE_URL: process.env.STORYBOOK_PUBLIC_API_BASE_URL,
	};
}

export function serializePublicRuntimeEnv(env: PublicRuntimeEnv): string {
	return JSON.stringify(env)
		.replace(/</g, "\\u003c")
		.replace(/\u2028/g, "\\u2028")
		.replace(/\u2029/g, "\\u2029");
}

export function createPublicRuntimeEnvScript(env: PublicRuntimeEnv): string {
	return `<script>globalThis.__PUBLIC_ENV__ = Object.freeze(${serializePublicRuntimeEnv(env)});</script>`;
}
