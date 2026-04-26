import type { PageServerLoad } from "./$types";
import { loadAuthConfig } from "$lib/server/auth";

export const load: PageServerLoad = async (event) => {
	return {
		authConfig: await loadAuthConfig(event),
	};
};
