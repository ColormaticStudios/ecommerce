<script lang="ts">
	import { type API } from "$lib/api";
	import type { UserModel } from "$lib/models";
	import Alert from "$lib/components/alert.svelte";
	import Button from "$lib/components/Button.svelte";
	import { getProfile, userStore } from "$lib/user";
	import Password from "$lib/components/password.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
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
		<TextInput
			bind:value={username}
			type="text"
			name="username"
			placeholder="Username"
			required
		/>
		<TextInput
			bind:value={email}
			type="email"
			name="email"
			placeholder="Email"
			required
		/>
		<TextInput
			bind:value={name}
			type="text"
			name="name"
			placeholder="Name (optional)"
		/>
		<Password bind:value={password} name="password" placeholder="Password" />
		<Password bind:value={passwordMatcher} name="confirm_password" placeholder="Confirm Password" />
		{#if !doPasswordsMatch}
			<div class="w-full">
				<Alert
					message="Passwords do not match."
					tone="error"
					icon="bi-x-circle-fill"
					onClose={() => (doPasswordsMatch = true)}
				/>
			</div>
		{/if}
		{#if errorMessage}
			<div class="w-full">
				<Alert
					message={errorMessage}
					tone="error"
					icon="bi-x-circle-fill"
					onClose={() => (errorMessage = "")}
				/>
			</div>
		{/if}
		<Button variant="primary" size="large" type="submit">Create Account</Button>
	</form>
</div>
