import { type API } from "$lib/api";
import { parseProfile, type ProfileModel, type UserModel } from "$lib/models";
import { writable } from "svelte/store";

export class User implements UserModel {
	api: API;
	id: number;
	subject: string;
	username: string;
	email: string;
	name: string | null;
	role: string;
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
		role: string,
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
		this.role = role;
		this.profile_photo_url = profile_photo_url;
		this.created_at = created_at;
		this.updated_at = updated_at;
		this.deleted_at = deleted_at;
	}

	logOut() {
		this.api.removeToken();
		userStore.logout();
		location.reload();
	}
}

export async function getProfile(api: API): Promise<User | null> {
	let userData: ProfileModel;
	try {
		userData = await api.getProfile();
	} catch (err) {
		console.warn("Failed to retrieve profile. Session token may be expired.");
		console.error(err);
		return null;
	}

	if (userData) {
		const parsed = parseProfile(userData);
		const user = new User(
			api,
			parsed.id,
			parsed.subject,
			parsed.username,
			parsed.email,
			parsed.name,
			parsed.role,
			parsed.currency,
			parsed.profile_photo_url,
			parsed.created_at,
			parsed.updated_at,
			parsed.deleted_at
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
