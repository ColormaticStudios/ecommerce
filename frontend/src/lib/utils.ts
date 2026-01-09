export function formatPrice(price: number, currency: string = "USD") {
	return new Intl.NumberFormat("en-US", {
		style: "currency",
		currency: currency,
	}).format(price);
}
