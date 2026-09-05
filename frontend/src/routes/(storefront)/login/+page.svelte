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
	let oidcRedirecting = $state(false);
	let postLoginRedirect = $derived(sanitizeAuthRedirectPath(page.url.searchParams.get("redirect")));
	let localSignInEnabled = $derived(data.authConfig.local_sign_in_enabled);
	let oidcEnabled = $derived(data.authConfig.oidc_enabled);
	let oidcDisplayName = $derived(data.authConfig.oidc_display_name);
	let reauthMessage = $derived(
		page.url.searchParams.get("reason") === "reauth"
			? $localization.translate(
					"storefront.account.session_expired",
					"Your session expired. Please sign in again."
				)
			: page.url.searchParams.get("reason") === "oidc_cancelled"
				? $localization.translate(
						"storefront.account.oidc_cancelled",
						"Sign-in was cancelled. You can try again."
					)
				: page.url.searchParams.get("reason") === "oidc_failed"
					? $localization.translate(
							"storefront.account.oidc_failed",
							"We couldn't complete sign-in. Please try again."
						)
					: ""
	);
	let signupHref = $derived(
		postLoginRedirect === "/"
			? resolve("/signup")
			: `${resolve("/signup")}?redirect=${encodeURIComponent(postLoginRedirect)}`
	);

	function resolveRedirectHref(path: string): string {
		const url = new URL(path, "https://storefront.local");
		// @ts-expect-error Sanitized redirect targets can still be dynamic route strings.
		const resolvedPath = resolve(url.pathname);
		return `${resolvedPath}${url.search}${url.hash}`;
	}

	function continueWithOIDC() {
		if (oidcRedirecting) return;
		oidcRedirecting = true;
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
		<div class="mt-6 w-full">
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
	title={$localization.translate("storefront.account.welcome_back", "Welcome back")}
	{oidcEnabled}
	{oidcDisplayName}
	oidcLoading={oidcRedirecting}
	{localSignInEnabled}
	dividerLabel={$localization.translate(
		"storefront.account.or_sign_in_with_email",
		"or sign in with email"
	)}
	oidcDescription={$localization.translate(
		"storefront.account.redirect_to_provider",
		"You'll be redirected to {provider} to sign in.",
		{ provider: oidcDisplayName }
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
		<form class="flex w-full flex-col gap-4" onsubmit={submit}>
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-200" for="email">
				<span>{$localization.translate("storefront.account.email", "Email")}</span>
				<TextInput
					bind:value={email}
					id="email"
					type="email"
					name="email"
					autocomplete="email"
					class="mt-1"
					required
				/>
			</label>
			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-200" for="password">
					{$localization.translate("storefront.account.password", "Password")}
				</label>
				<Password
					bind:value={password}
					id="password"
					name="password"
					autocomplete="current-password"
					class="mt-1"
					required
				/>
			</div>
			<Button variant="primary" size="large" type="submit" class="mt-1 w-full text-base"
				>{$localization.translate("storefront.account.sign_in", "Sign in")}</Button
			>
		</form>
		<p class="text-center text-sm text-gray-600 dark:text-gray-300">
			{$localization.translate("storefront.account.new_here", "New here?")}
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a class="ml-1 font-medium text-blue-600 hover:underline dark:text-blue-400" href={signupHref}
				>{$localization.translate("storefront.account.create_account_link", "Create an account")}</a
			>
		</p>
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
