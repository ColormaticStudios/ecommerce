import { expect, type APIRequestContext, type Page } from "@playwright/test";

export const apiBaseURL = process.env.PUBLIC_API_BASE_URL || "http://127.0.0.1:3001";

export type SeededUser = {
	id: number;
	email: string;
	username: string;
	name: string;
	role: "admin" | "customer";
};

export type SeededBrand = {
	id: number;
	name: string;
	slug: string;
	description: string | null;
	is_active: boolean;
};

export async function establishSession(page: Page, email: string): Promise<void> {
	await page.goto(`${apiBaseURL}/__test/login?email=${encodeURIComponent(email)}`);
}

export async function seedTestUser(
	request: APIRequestContext,
	input: {
		email: string;
		username: string;
		name?: string;
		role?: "admin" | "customer";
	}
): Promise<SeededUser> {
	const response = await request.post(`${apiBaseURL}/__test/users`, {
		data: {
			email: input.email,
			username: input.username,
			name: input.name,
			role: input.role ?? "customer",
		},
	});

	expect(response.ok()).toBeTruthy();
	return (await response.json()) as SeededUser;
}

export async function seedAndLoginUser(
	page: Page,
	request: APIRequestContext,
	input: {
		email: string;
		username: string;
		name?: string;
		role?: "admin" | "customer";
	}
): Promise<SeededUser> {
	const user = await seedTestUser(request, input);
	await establishSession(page, user.email);
	return user;
}

export async function seedTestBrand(
	request: APIRequestContext,
	input: {
		name: string;
		slug: string;
		description?: string;
		is_active?: boolean;
	}
): Promise<SeededBrand> {
	const response = await request.post(`${apiBaseURL}/__test/brands`, {
		data: input,
	});

	expect(response.ok()).toBeTruthy();
	return (await response.json()) as SeededBrand;
}
