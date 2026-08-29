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
	import { getLocalizedApiErrorMessage } from "$lib/api/errors";
	import { LOCALIZATION_CONTEXT, type LocalizationRuntime } from "$lib/localization/runtime";

	let api: API = getContext("api");
	const localization = getContext<LocalizationRuntime>(LOCALIZATION_CONTEXT);

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
			? $localization.translate(
					"storefront.account.session_expired",
					"Your session expired. Please sign in again."
				)
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
			errorMessage = getLocalizedApiErrorMessage(
				err,
				localization.translate.bind(localization),
				$localization.translate(
					"storefront.account.invalid_credentials",
					"Invalid email or password."
				)
			);
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

{#snippet authAlerts()}
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
{/snippet}

<AuthFormShell
	title={$localization.translate("storefront.account.log_in", "Log In")}
	{oidcEnabled}
	oidcDescription={$localization.translate(
		"storefront.account.provider_sign_in",
		"Use your identity provider to sign in without a local password."
	)}
	showUnavailable={!localSignInEnabled && !oidcEnabled}
	unavailableMessage={$localization.translate(
		"storefront.account.sign_in_unavailable",
		"Sign-in is currently unavailable."
	)}
	onOidc={continueWithOIDC}
	alerts={authAlerts}
>
	{#if localSignInEnabled}
		<form class="contents" onsubmit={submit}>
			<TextInput
				bind:value={email}
				type="email"
				name="email"
				placeholder={$localization.translate("storefront.account.email", "Email")}
				required
			/>
			<Password
				bind:value={password}
				name="password"
				placeholder={$localization.translate("storefront.account.password", "Password")}
			/>
			<Button variant="primary" size="large" type="submit"
				>{$localization.translate("storefront.account.log_in", "Log In")}</Button
			>
		</form>
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
