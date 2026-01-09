<script lang="ts">
	import { type API } from "$lib/api";
	import type { UserModel } from "$lib/models";
	import { getProfile, userStore } from "$lib/user";
	import Password from "$lib/components/password.svelte";
	import { getContext } from "svelte";
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";

	let api: API = getContext("api");

	let username = $state("");
	let email = $state("");
	let password = $state("");
	let name = $state("");

	let passwordMatcher = $state("");
	let doPasswordsMatch = $state(true);

	async function submit(event: SubmitEvent) {
		event.preventDefault();

		if (password !== passwordMatcher) {
			doPasswordsMatch = false;
			return;
		}

		interface Response {
			token?: string;
			user?: UserModel;
		}
		let response: Response = {};
		try {
			response = api.register({
				username: username,
				email: email,
				password: password,
				name: name,
			}) as Response;
		} catch (err) {
			console.error(err);
			return;
		}

		api.setToken(response.token);
		let user = await getProfile(api);
		if (user) {
			userStore.setUser(user);
			goto(resolve("/"));
		} else {
			console.error("Failed to log in");
		}
	}
</script>

<div class="mt-[10%] flex flex-col items-center justify-center">
	<h1 class="text-4xl font-bold">Sign Up</h1>
	<form
		class="m-4 flex w-sm flex-col items-center justify-center gap-2 rounded-lg border border-gray-300 bg-gray-100 p-4 dark:border-gray-600 dark:bg-gray-700"
		onsubmit={submit}
	>
		<input
			bind:value={username}
			class="textinput"
			type="text"
			name="username"
			placeholder="Username"
			required
		/>
		<input
			bind:value={email}
			class="textinput"
			type="email"
			name="email"
			placeholder="Email"
			required
		/>
		<input
			bind:value={name}
			class="textinput"
			type="text"
			name="name"
			placeholder="Name (optional)"
		/>
		<Password bind:value={password} name="password" placeholder="Password" />
		<Password bind:value={passwordMatcher} name="confirm_password" placeholder="Confirm Password" />
		{#if !doPasswordsMatch}
			<p class="float-left text-red-500">Passwords do not match.</p>
		{/if}
		<input class="btn btn-large btn-primary" type="submit" value="Create Account" />
	</form>
</div>
