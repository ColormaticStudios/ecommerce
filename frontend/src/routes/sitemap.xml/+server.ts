import type { RequestHandler } from "./$types";
import { serverRequest } from "$lib/server/api";

export const GET: RequestHandler = async (event) => {
	const xml = await serverRequest<string>(event, "/content/sitemap.xml");
	return new Response(xml, {
		headers: {
			"Content-Type": "application/xml; charset=utf-8",
			"Cache-Control": "public, max-age=900",
		},
	});
};
