<script lang="ts">
	import { type API } from "$lib/api";
	import { sanitizeAuthRedirectPath } from "$lib/auth";
	import Alert from "$lib/components/Alert.svelte";
	import Button from "$lib/components/Button.svelte";
	import Card from "$lib/components/Card.svelte";
	import { getProfile, userStore } from "$lib/user";
	import Password from "$lib/components/Password.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import { getContext } from "svelte";
	import { resolve } from "$app/paths";
	import { page } from "$app/state";
	import type { PageData } from "./$types";

	let api: API = getContext("api");

	interface Props {
		data: PageData;
	}

	let { data }: Props = $props();

	let email = $state("");
	let password = $state("");
	let errorMessage = $state("");
	let postLoginRedirect = $derived(sanitizeAuthRedirectPath(page.url.searchParams.get("redirect")));
	let localSignInEnabled = $derived(data.authConfig.local_sign_in_enabled);
	let oidcEnabled = $derived(data.authConfig.oidc_enabled);
	let reauthMessage = $derived(
		page.url.searchParams.get("reason") === "reauth"
			? "Your session expired. Please sign in again."
			: ""
	);

	function resolveRedirectHref(path: string): string {
		const url = new URL(path, "https://storefront.local");
		// @ts-expect-error Sanitized redirect targets can still be dynamic route strings.
		const resolvedPath = resolve(url.pathname);
		return `${resolvedPath}${url.search}${url.hash}`;
	}

	function continueWithOIDC() {
		window.location.assign(api.buildOIDCLoginURL(postLoginRedirect));
	}

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
			window.location.assign(resolveRedirectHref(postLoginRedirect));
		} else {
			console.error("Failed to log in");
		}
	}
</script>

<div class="mt-[10%] flex flex-col items-center justify-center">
	<h1 class="text-4xl font-bold">Log In</h1>
	{#if reauthMessage}
		<div class="m-4 w-sm">
			<Alert
				message={reauthMessage}
				tone="error"
				icon="bi-shield-exclamation"
				onClose={undefined}
			/>
		</div>
	{/if}
	<Card
		radius="lg"
		padding="sm"
		class="m-4 flex w-sm flex-col items-center justify-center gap-2 bg-gray-100 dark:bg-gray-900"
	>
		{#if oidcEnabled}
			<Button
				variant="regular"
				size="large"
				type="button"
				class="flex w-full items-center justify-center gap-2"
				onclick={continueWithOIDC}
			>
				<i class="bi bi-shield-lock" aria-hidden="true"></i>
				<span>Continue with OpenID Connect</span>
			</Button>
			<p class="w-full text-center text-sm text-gray-600 dark:text-gray-300">
				Use your identity provider to sign in without a local password.
			</p>
			<div
				class="flex w-full items-center gap-3 py-1 text-xs tracking-[0.3em] text-gray-500 uppercase"
			>
				<div class="h-px flex-1 bg-gray-300 dark:bg-gray-700"></div>
				<span>Or</span>
				<div class="h-px flex-1 bg-gray-300 dark:bg-gray-700"></div>
			</div>
		{/if}
		{#if localSignInEnabled}
			<form class="contents" onsubmit={submit}>
				<TextInput bind:value={email} type="email" name="email" placeholder="Email" required />
				<Password bind:value={password} name="password" placeholder="Password" />
				<Button variant="primary" size="large" type="submit">Log In</Button>
			</form>
		{:else if !oidcEnabled}
			<div class="w-full">
				<Alert
					message="Sign-in is currently unavailable."
					tone="error"
					icon="bi-shield-exclamation"
					onClose={undefined}
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
	</Card>
</div>
