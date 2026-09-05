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

	let username = $state("");
	let email = $state("");
	let password = $state("");
	let name = $state("");

	let passwordMatcher = $state("");
	let doPasswordsMatch = $state(true);
	let errorMessage = $state("");
	let oidcRedirecting = $state(false);
	let postSignupRedirect = $derived(
		sanitizeAuthRedirectPath(page.url.searchParams.get("redirect"))
	);
	let localSignInEnabled = $derived(data.authConfig.local_sign_in_enabled);
	let oidcEnabled = $derived(data.authConfig.oidc_enabled);
	let oidcDisplayName = $derived(data.authConfig.oidc_display_name);
	let loginHref = $derived(
		postSignupRedirect === "/"
			? resolve("/login")
			: `${resolve("/login")}?redirect=${encodeURIComponent(postSignupRedirect)}`
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
			errorMessage = getLocalizedApiErrorMessage(
				err,
				localization.translate.bind(localization),
				$localization.translate(
					"storefront.account.create_error",
					"Unable to create account. Please check your details."
				)
			);
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
	title={$localization.translate("storefront.account.create_account_title", "Create your account")}
	{oidcEnabled}
	{oidcDisplayName}
	oidcLoading={oidcRedirecting}
	{localSignInEnabled}
	dividerLabel={$localization.translate(
		"storefront.account.or_create_with_email",
		"or create an account with email"
	)}
	oidcDescription={$localization.translate(
		"storefront.account.provider_create_reassurance",
		"New to the store? Your account will be created after you sign in."
	)}
	showUnavailable={!localSignInEnabled && !oidcEnabled}
	unavailableMessage={$localization.translate(
		"storefront.account.creation_unavailable",
		"Account creation is currently unavailable."
	)}
	onOidc={continueWithOIDC}
>
	{#if localSignInEnabled}
		<form class="flex w-full flex-col gap-4" onsubmit={submit}>
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-200" for="username">
				<span>{$localization.translate("storefront.account.username", "Username")}</span>
				<TextInput
					bind:value={username}
					id="username"
					type="text"
					name="username"
					autocomplete="username"
					class="mt-1"
					required
				/>
			</label>
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-200" for="signup-email">
				<span>{$localization.translate("storefront.account.email", "Email")}</span>
				<TextInput
					bind:value={email}
					id="signup-email"
					type="email"
					name="email"
					autocomplete="email"
					class="mt-1"
					required
				/>
			</label>
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-200" for="name">
				<span>{$localization.translate("storefront.account.name_optional", "Name (optional)")}</span
				>
				<TextInput
					bind:value={name}
					id="name"
					type="text"
					name="name"
					autocomplete="name"
					class="mt-1"
				/>
			</label>
			<div>
				<label
					class="block text-sm font-medium text-gray-700 dark:text-gray-200"
					for="new-password"
				>
					{$localization.translate("storefront.account.password", "Password")}
				</label>
				<Password
					bind:value={password}
					id="new-password"
					name="password"
					autocomplete="new-password"
					class="mt-1"
					required
				/>
			</div>
			<div>
				<label
					class="block text-sm font-medium text-gray-700 dark:text-gray-200"
					for="confirm-password"
				>
					{$localization.translate("storefront.account.confirm_password", "Confirm Password")}
				</label>
				<Password
					bind:value={passwordMatcher}
					id="confirm-password"
					name="confirm_password"
					autocomplete="new-password"
					class="mt-1"
					required
				/>
			</div>
			<Button variant="primary" size="large" type="submit" class="mt-1 w-full text-base"
				>{$localization.translate("storefront.account.create_account", "Create Account")}</Button
			>
		</form>
		<p class="text-center text-sm text-gray-600 dark:text-gray-300">
			{$localization.translate(
				"storefront.account.already_have_account",
				"Already have an account?"
			)}
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a class="ml-1 font-medium text-blue-600 hover:underline dark:text-blue-400" href={loginHref}
				>{$localization.translate("storefront.account.sign_in_link", "Sign in")}</a
			>
		</p>
	{/if}
	{#if !doPasswordsMatch}
		<div class="w-full">
			<Alert
				message={$localization.translate(
					"storefront.account.passwords_mismatch",
					"Passwords do not match."
				)}
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
