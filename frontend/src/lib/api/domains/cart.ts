import type { components } from "$lib/api/generated/openapi";
import { parseCart, parseCartItem, type CartItemModel, type CartModel } from "$lib/models";

type RequestFn = <T>(
	method: string,
	path: string,
	data?: object,
	params?: Record<string, unknown>
) => Promise<T>;

type CartPayload = components["schemas"]["Cart"];
type CartItemPayload = components["schemas"]["CartItem"];
type MessageResponse = components["schemas"]["MessageResponse"];

export async function viewCart(request: RequestFn): Promise<CartModel> {
	const response = await request<CartPayload>("GET", "/me/cart");
	return parseCart(response);
}

export async function addToCart(
	request: RequestFn,
	data: components["schemas"]["AddCartItemRequest"]
): Promise<CartModel> {
	const response = await request<CartPayload>("POST", "/me/cart", data);
	return parseCart(response);
}

export async function updateCartItem(
	request: RequestFn,
	itemId: number,
	data: components["schemas"]["UpdateCartItemRequest"]
): Promise<CartItemModel> {
	const response = await request<CartItemPayload>("PATCH", `/me/cart/${itemId}`, data);
	return parseCartItem(response);
}

export async function removeCartItem(request: RequestFn, itemId: number): Promise<MessageResponse> {
	return request("DELETE", `/me/cart/${itemId}`);
}
