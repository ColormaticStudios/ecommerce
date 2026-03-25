import type { RequestOptions } from "$lib/api";
import type { components, paths } from "$lib/api/generated/openapi";
import { parseOrder, type OrderModel } from "$lib/models";

type RequestFn = <T>(
	method: string,
	path: string,
	data?: object,
	params?: Record<string, unknown>,
	options?: RequestOptions
) => Promise<T>;

type CreateOrderRequest = components["schemas"]["CreateCheckoutOrderRequest"];
type AuthorizeCheckoutOrderPaymentRequest =
	components["schemas"]["AuthorizeCheckoutOrderPaymentRequest"];
type ProcessPaymentResponse = components["schemas"]["ProcessPaymentResponse"];
type CheckoutPluginCatalog = components["schemas"]["CheckoutPluginCatalog"];
type CheckoutQuoteRequest = components["schemas"]["CheckoutQuoteRequest"];
type CheckoutQuoteResponse = components["schemas"]["CheckoutQuoteResponse"];
type OrderPayload = components["schemas"]["Order"];
type OrderPagePayload = components["schemas"]["OrderPage"];
type ListUserOrdersQuery = paths["/api/v1/me/orders"]["get"]["parameters"]["query"];

type ListOrdersParams = Omit<NonNullable<ListUserOrdersQuery>, "status"> & {
	status?: NonNullable<ListUserOrdersQuery>["status"] | "";
};

export async function createOrder(
	request: RequestFn,
	data: CreateOrderRequest,
	idempotencyKey?: string
): Promise<OrderModel> {
	const response = await request<OrderPayload>("POST", "/checkout/orders", data, undefined, {
		headers: idempotencyKey ? { "Idempotency-Key": idempotencyKey } : undefined,
	});
	return parseOrder(response);
}

export async function processPayment(
	request: RequestFn,
	orderId: number,
	data: AuthorizeCheckoutOrderPaymentRequest,
	idempotencyKey?: string
): Promise<OrderModel> {
	const response = await request<ProcessPaymentResponse>(
		"POST",
		`/checkout/orders/${orderId}/payments/authorize`,
		data,
		undefined,
		{
			headers: idempotencyKey ? { "Idempotency-Key": idempotencyKey } : undefined,
		}
	);
	return parseOrder(response.order);
}

export async function listCheckoutPlugins(request: RequestFn): Promise<CheckoutPluginCatalog> {
	return request<CheckoutPluginCatalog>("GET", "/checkout/plugins");
}

export async function quoteCheckout(
	request: RequestFn,
	data: CheckoutQuoteRequest
): Promise<CheckoutQuoteResponse> {
	return request<CheckoutQuoteResponse>("POST", "/checkout/quote", data);
}

export async function listOrders(
	request: RequestFn,
	params?: ListOrdersParams
): Promise<{ data: OrderModel[]; pagination: OrderPagePayload["pagination"] }> {
	const query = {
		...params,
		status: params?.status === "" ? undefined : params?.status,
	};
	const response = await request<OrderPagePayload>("GET", "/me/orders", undefined, query);
	return { data: response.data.map(parseOrder), pagination: response.pagination };
}

export async function getOrderDetails(request: RequestFn, orderId: number): Promise<OrderModel> {
	const response = await request<OrderPayload>("GET", `/me/orders/${orderId}`);
	return parseOrder(response);
}

export async function cancelOrder(request: RequestFn, orderId: number): Promise<OrderModel> {
	const response = await request<OrderPayload>("POST", `/me/orders/${orderId}/cancel`);
	return parseOrder(response);
}
