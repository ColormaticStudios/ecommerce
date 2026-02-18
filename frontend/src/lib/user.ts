import { type API } from "$lib/api";
import { type UserModel } from "$lib/models";
import { writable } from "svelte/store";

export class User implements UserModel {
	api: API;
	id: number;
	subject: string;
	username: string;
	email: string;
	name: string | null;
	role: "admin" | "customer";
	currency: string;
	profile_photo_url: string | null;
	created_at: Date;
	updated_at: Date;
	deleted_at: Date | null;

	constructor(
		api: API,
		id: number,
		subject: string,
			username: string,
			email: string,
			name: string | null,
			role: "admin" | "customer",
			currency: string,
		profile_photo_url: string | null,
		created_at: Date,
		updated_at: Date,
		deleted_at: Date | null
	) {
		this.api = api;
		this.id = id;
		this.subject = subject;
		this.username = username;
		this.email = email;
		this.name = name;
		this.role = role;
		this.currency = currency;
		this.profile_photo_url = profile_photo_url;
		this.created_at = created_at;
		this.updated_at = updated_at;
		this.deleted_at = deleted_at;
	}

	async logOut() {
		try {
			await this.api.logout();
		} catch (err) {
			console.error(err);
		}
		userStore.logout();
		location.reload();
	}
}

export async function getProfile(api: API): Promise<User | null> {
	let userData: UserModel;
	try {
		userData = await api.getProfile();
	} catch (err) {
		const error = err as { status?: number };
		if (error.status !== 401) {
			console.error(err);
		}
		return null;
	}

	if (userData) {
		const user = new User(
			api,
			userData.id,
			userData.subject,
			userData.username,
			userData.email,
			userData.name,
			userData.role,
			userData.currency,
			userData.profile_photo_url,
			userData.created_at,
			userData.updated_at,
			userData.deleted_at
		);
		return user;
	}
	return null;
}

function createUserStore() {
	const { subscribe, set } = writable<User | null>(null);

	return {
		subscribe,

		// Called on app startup/layout init
		async load(api: API) {
			const user = await getProfile(api);
			set(user);
			return user;
		},

		// Called after login/account creation
		setUser(user: User) {
			set(user);
		},

		logout() {
			set(null);
		},
	};
}
export const userStore = createUserStore();
