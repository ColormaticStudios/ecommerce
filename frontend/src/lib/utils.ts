export function formatPrice(price: number, currency: string = "USD") {
	return new Intl.NumberFormat("en-US", {
		style: "currency",
		currency: currency,
	}).format(price);
}

export interface PriceDisplayInput {
	price?: number | null;
	base_price?: number | null;
	discount_amount?: number | null;
	final_price?: number | null;
}

export function displayUnitPrice(input: PriceDisplayInput): number {
	return input.final_price ?? input.price ?? 0;
}

export function displayBasePrice(input: PriceDisplayInput): number {
	return input.base_price ?? input.price ?? 0;
}

export function hasDiscount(input: PriceDisplayInput): boolean {
	return (input.discount_amount ?? 0) > 0 && displayUnitPrice(input) < displayBasePrice(input);
}
