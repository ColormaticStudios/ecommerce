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
type CartSummaryPayload = components["schemas"]["CheckoutCartSummary"];
type MessageResponse = components["schemas"]["MessageResponse"];

export async function viewCart(request: RequestFn): Promise<CartModel> {
	const response = await request<CartPayload>("GET", "/checkout/cart");
	return parseCart(response);
}

export async function viewCartSummary(request: RequestFn): Promise<number> {
	const response = await request<CartSummaryPayload>("GET", "/checkout/cart/summary");
	return response.item_count;
}

export async function addToCart(
	request: RequestFn,
	data: components["schemas"]["AddCartItemRequest"]
): Promise<CartModel> {
	const response = await request<CartPayload>("POST", "/checkout/cart/items", data);
	return parseCart(response);
}

export async function updateCartItem(
	request: RequestFn,
	itemId: number,
	data: components["schemas"]["UpdateCartItemRequest"]
): Promise<CartItemModel> {
	const response = await request<CartItemPayload>("PATCH", `/checkout/cart/items/${itemId}`, data);
	return parseCartItem(response);
}

export async function removeCartItem(request: RequestFn, itemId: number): Promise<MessageResponse> {
	return request("DELETE", `/checkout/cart/items/${itemId}`);
}
