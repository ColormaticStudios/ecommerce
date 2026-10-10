import type { Meta, StoryObj } from "@storybook/sveltekit";
import { expect, fn, userEvent, within } from "storybook/test";
import ProductCard from "./ProductCard.svelte";
const clicked = fn((event: MouseEvent) => event.preventDefault());
const meta = {
	title: "Components/Product card",
	component: ProductCard,
	args: {
		href: "/product/101",
		data: {
			name: "Field Jacket",
			brand: "Colormatic",
			description: "An everyday outer layer.",
			price: 129,
			stock: 8,
		},
		onclick: clicked,
	},
} satisfies Meta<typeof ProductCard>;
export default meta;
type Story = StoryObj<typeof meta>;
export const ClickThrough: Story = {
	play: async ({ canvasElement }) => {
		clicked.mockClear();
		await userEvent.click(within(canvasElement).getByRole("link"));
		await expect(clicked).toHaveBeenCalledOnce();
	},
};
