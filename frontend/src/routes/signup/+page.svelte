<script lang="ts">
	import { type API } from "$lib/api";
	import { sanitizeAuthRedirectPath } from "$lib/auth";
	import Alert from "$lib/components/Alert.svelte";
	import AuthFormShell from "$lib/components/AuthFormShell.svelte";
	import Button from "$lib/components/Button.svelte";
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

	let username = $state("");
	let email = $state("");
	let password = $state("");
	let name = $state("");

	let passwordMatcher = $state("");
	let doPasswordsMatch = $state(true);
	let errorMessage = $state("");
	let postSignupRedirect = $derived(
		sanitizeAuthRedirectPath(page.url.searchParams.get("redirect"))
	);
	let localSignInEnabled = $derived(data.authConfig.local_sign_in_enabled);
	let oidcEnabled = $derived(data.authConfig.oidc_enabled);

	function resolveRedirectHref(path: string): string {
		const url = new URL(path, "https://storefront.local");
		// @ts-expect-error Sanitized redirect targets can still be dynamic route strings.
		const resolvedPath = resolve(url.pathname);
		return `${resolvedPath}${url.search}${url.hash}`;
	}

	function continueWithOIDC() {
		window.location.assign(api.buildOIDCLoginURL(postSignupRedirect));
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		errorMessage = "";

		if (password !== passwordMatcher) {
			doPasswordsMatch = false;
			return;
		}

		try {
			await api.register({
				username: username,
				email: email,
				password: password,
				name: name,
			});
		} catch (err) {
			const error = err as { body?: { error?: string } };
			errorMessage = error.body?.error ?? "Unable to create account. Please check your details.";
			console.error(err);
			return;
		}

		let user = await getProfile(api);
		if (user) {
			userStore.setUser(user);
			window.location.assign(resolveRedirectHref(postSignupRedirect));
		} else {
			console.error("Failed to log in");
		}
	}
</script>

<AuthFormShell
	title="Sign Up"
	{oidcEnabled}
	oidcDescription="Your account will be created automatically the first time your provider signs you in."
	showUnavailable={!localSignInEnabled && !oidcEnabled}
	unavailableMessage="Account creation is currently unavailable."
	onOidc={continueWithOIDC}
>
	{#if localSignInEnabled}
		<form class="contents" onsubmit={submit}>
			<TextInput
				bind:value={username}
				type="text"
				name="username"
				placeholder="Username"
				required
			/>
			<TextInput bind:value={email} type="email" name="email" placeholder="Email" required />
			<TextInput bind:value={name} type="text" name="name" placeholder="Name (optional)" />
			<Password bind:value={password} name="password" placeholder="Password" />
			<Password
				bind:value={passwordMatcher}
				name="confirm_password"
				placeholder="Confirm Password"
			/>
			<Button variant="primary" size="large" type="submit">Create Account</Button>
		</form>
	{/if}
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
</AuthFormShell>
