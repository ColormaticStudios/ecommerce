import { type API } from "$lib/api";
import { type ProfileModel, type UserModel } from "$lib/models";
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

	constructor(
		api: API,
		id: number,
		subject: string,
		username: string,
		email: string,
		name: string | null,
		role: string,
		currency: string,
		profile_photo_url: string | null
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
		console.dir(userData);
	} catch (err) {
		console.warn("Session token is expired.");
		console.log(err);
		return null;
	}

	if (userData) {
		const user = new User(
			api,
			userData.ID,
			userData.Subject,
			userData.Username,
			userData.Email,
			userData.name,
			userData.role,
			userData.currency,
			userData.profile_photo_url
		);
		console.dir(user);
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
