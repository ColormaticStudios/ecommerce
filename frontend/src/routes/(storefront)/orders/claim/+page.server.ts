import type { PageServerLoad } from "./$types";
import { serverIsAuthenticated } from "$lib/server/api";

export const load: PageServerLoad = async (event) => {
	const isAuthenticated = await serverIsAuthenticated(event);

	return {
		isAuthenticated,
		initialEmail: event.url.searchParams.get("email") ?? "",
		initialToken: event.url.searchParams.get("token") ?? "",
	};
};
