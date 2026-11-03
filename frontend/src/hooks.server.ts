import { json, type Handle } from "@sveltejs/kit";
import { createPublicRuntimeEnvScript, readServerPublicRuntimeEnv } from "$lib/public-env";
import { isPublicStorefront, preparePublicLocalization } from "$lib/server/public-localization";
import { PRIVATE_CACHE_CONTROL } from "$lib/server/cache";

const PUBLIC_RUNTIME_ENV_MARKER = "%PUBLIC_RUNTIME_ENV%";

export const handle: Handle = async ({ event, resolve }) => {
	if (isPublicStorefront(event)) {
		let target: string | null;
		try {
			target = await preparePublicLocalization(event);
		} catch (err) {
			console.error("Failed to resolve public page localization", err);
			return new Response("Service unavailable", {
				status: 503,
				headers: { "Cache-Control": PRIVATE_CACHE_CONTROL },
			});
		}
		if (target) {
			// Hook redirects bypass resolve(), where SvelteKit applies setHeaders.
			// Return the data-navigation envelope directly to retain its cache policy.
			if (event.isDataRequest)
				return json(
					{ type: "redirect", location: target },
					{ headers: { "Cache-Control": PRIVATE_CACHE_CONTROL } }
				);
			return new Response(null, {
				status: 307,
				headers: { Location: target, "Cache-Control": PRIVATE_CACHE_CONTROL },
			});
		}
	}
	return resolve(event, {
		transformPageChunk: ({ html }) =>
			html.includes(PUBLIC_RUNTIME_ENV_MARKER)
				? html.replace(
						PUBLIC_RUNTIME_ENV_MARKER,
						createPublicRuntimeEnvScript(readServerPublicRuntimeEnv())
					)
				: html,
	});
};
