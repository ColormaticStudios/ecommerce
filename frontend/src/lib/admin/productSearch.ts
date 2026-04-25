import type { API } from "$lib/api";
import type { ProductModel } from "$lib/models";

export async function searchAdminProducts(
	api: API,
	query: string,
	limit: number
): Promise<ProductModel[]> {
	const response = await api.listAdminProducts({
		q: query.trim() || undefined,
		page: 1,
		limit,
	});
	return response.data;
}
