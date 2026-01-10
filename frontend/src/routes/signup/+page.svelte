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
	let errorMessage = $state("");

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		errorMessage = "";

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
			response = (await api.register({
				username: username,
				email: email,
				password: password,
				name: name,
			})) as Response;
		} catch (err) {
			const error = err as { body?: { error?: string } };
			errorMessage = error.body?.error ?? "Unable to create account. Please check your details.";
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
		class="m-4 flex w-sm flex-col items-center justify-center gap-2 rounded-lg border border-gray-300 bg-gray-100 p-4 dark:border-gray-800 dark:bg-gray-900"
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
		{#if errorMessage}
			<p
				class="w-full rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/40 dark:text-red-200"
			>
				<i class="bi bi-exclamation-triangle-fill mr-1"></i>
				{errorMessage}
			</p>
		{/if}
		<input class="btn btn-large btn-primary" type="submit" value="Create Account" />
	</form>
</div>
