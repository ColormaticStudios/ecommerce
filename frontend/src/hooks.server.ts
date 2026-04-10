import type { Handle } from "@sveltejs/kit";
import { createPublicRuntimeEnvScript, readServerPublicRuntimeEnv } from "$lib/public-env";

const PUBLIC_RUNTIME_ENV_MARKER = "%PUBLIC_RUNTIME_ENV%";

export const handle: Handle = async ({ event, resolve }) =>
	resolve(event, {
		transformPageChunk: ({ html }) =>
			html.includes(PUBLIC_RUNTIME_ENV_MARKER)
				? html.replace(
						PUBLIC_RUNTIME_ENV_MARKER,
						createPublicRuntimeEnvScript(readServerPublicRuntimeEnv())
					)
				: html,
	});
