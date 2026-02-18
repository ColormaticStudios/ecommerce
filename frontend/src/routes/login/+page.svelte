<script lang="ts">
	import { type API } from "$lib/api";
	import Alert from "$lib/components/alert.svelte";
	import Button from "$lib/components/Button.svelte";
	import { getProfile, userStore } from "$lib/user";
	import Password from "$lib/components/password.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import { getContext } from "svelte";
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";

	let api: API = getContext("api");

	let email = $state("");
	let password = $state("");
	let errorMessage = $state("");

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		errorMessage = "";

		try {
			await api.login({
				email: email,
				password: password,
			});
		} catch (err) {
			const error = err as { body?: { error?: string } };
			errorMessage = error.body?.error ?? "Invalid email or password.";
			console.error(err);
			return;
		}

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
	<h1 class="text-4xl font-bold">Log In</h1>
	<form
		class="m-4 flex w-sm flex-col items-center justify-center gap-2 rounded-lg border border-gray-300 bg-gray-100 p-4 dark:border-gray-800 dark:bg-gray-900"
		onsubmit={submit}
	>
		<TextInput
			bind:value={email}
			type="email"
			name="email"
			placeholder="Email"
			required
		/>
		<Password bind:value={password} name="password" placeholder="Password" />
		<Button variant="primary" size="large" type="submit">Log In</Button>
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
	</form>
</div>
