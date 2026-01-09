import type { ParamMatcher } from "@sveltejs/kit";

export const match = ((param: string): boolean => {
	const parsedId = Number(param);
	return Number.isInteger(parsedId);
}) satisfies ParamMatcher;
