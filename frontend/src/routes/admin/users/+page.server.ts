import type { PageServerLoad } from "./$types";
import { parseProfile, type UserModel } from "$lib/models";
import { serverRequest } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";

type UserPagePayload = components["schemas"]["UserPage"];

const defaultAdminPageLimit = 10;

export const load: PageServerLoad = async (event) => {
	const { isAdmin } = await event.parent();

	let users: UserModel[] = [];
	const userPage = 1;
	let userTotalPages = 1;
	const userLimit = defaultAdminPageLimit;
	let userTotal = 0;
	let errorMessage = "";

	if (!isAdmin) {
		return {
			users,
			userPage,
			userTotalPages,
			userLimit,
			userTotal,
			errorMessage,
		};
	}

	try {
		const payload = await serverRequest<UserPagePayload>(event, "/admin/users", {
			page: userPage,
			limit: userLimit,
		});
		users = payload.data.map(parseProfile);
		userTotalPages = Math.max(1, payload.pagination.total_pages);
		userTotal = payload.pagination.total;
	} catch (err) {
		console.error(err);
		errorMessage = "Unable to load users.";
	}

	return {
		users,
		userPage,
		userTotalPages,
		userLimit,
		userTotal,
		errorMessage,
	};
};
