import type { LayoutServerLoad } from "./$types";
import { loadAdminAccess } from "$lib/admin/server";

export const load: LayoutServerLoad = async (event) => {
	return await loadAdminAccess(event);
};
