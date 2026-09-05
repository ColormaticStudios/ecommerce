<script lang="ts">
	import Alert from "$lib/components/Alert.svelte";
	import Button from "$lib/components/Button.svelte";
	import Card from "$lib/components/Card.svelte";
	import { getContext } from "svelte";
	import { LOCALIZATION_CONTEXT, type LocalizationRuntime } from "$lib/localization/runtime";

	const localization = getContext<LocalizationRuntime>(LOCALIZATION_CONTEXT);

	interface Props {
		title: string;
		subtitle?: string;
		oidcEnabled?: boolean;
		oidcDisplayName?: string;
		oidcDescription?: string;
		oidcLoading?: boolean;
		localSignInEnabled?: boolean;
		dividerLabel?: string;
		unavailableMessage?: string;
		showUnavailable?: boolean;
		onOidc?: () => void;
		children?: import("svelte").Snippet;
		alerts?: import("svelte").Snippet;
	}

	let {
		title,
		subtitle = "",
		oidcEnabled = false,
		oidcDisplayName = "",
		oidcDescription = "",
		oidcLoading = false,
		localSignInEnabled = true,
		dividerLabel = "",
		unavailableMessage = "",
		showUnavailable = false,
		onOidc,
		children,
		alerts,
	}: Props = $props();
</script>

<section class="mx-auto flex w-full max-w-md flex-col px-4 py-10 sm:py-16">
	<header class="text-center">
		<h1 class="text-3xl font-bold tracking-tight text-gray-950 dark:text-white">{title}</h1>
		{#if subtitle}
			<p class="mt-2 text-base text-gray-600 dark:text-gray-300">{subtitle}</p>
		{/if}
	</header>
	{@render alerts?.()}
	<Card radius="2xl" padding="lg" shadow="md" class="mt-8 flex w-full min-w-0 flex-col gap-5">
		{#if oidcEnabled}
			<Button
				variant={localSignInEnabled ? "regular" : "primary"}
				size="large"
				type="button"
				class="flex min-h-12 w-full items-center justify-center gap-2 text-base"
				onclick={onOidc}
				disabled={oidcLoading}
				aria-busy={oidcLoading}
			>
				<i
					class={oidcLoading ? "bi bi-arrow-repeat animate-spin" : "bi bi-arrow-right"}
					aria-hidden="true"
				></i>
				<span
					>{oidcLoading
						? $localization.translate(
								"storefront.account.redirecting_to_provider",
								"Redirecting to {provider}...",
								{ provider: oidcDisplayName }
							)
						: $localization.translate(
								"storefront.account.continue_to_provider",
								"Continue to {provider}",
								{ provider: oidcDisplayName }
							)}</span
				>
			</Button>
			{#if oidcDescription}
				<p class="w-full text-center text-sm text-gray-600 dark:text-gray-300">
					{oidcDescription}
				</p>
			{/if}
			{#if localSignInEnabled}
				<div class="flex w-full items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
					<div class="h-px flex-1 bg-gray-200 dark:bg-gray-700"></div>
					<span>{dividerLabel}</span>
					<div class="h-px flex-1 bg-gray-200 dark:bg-gray-700"></div>
				</div>
			{/if}
		{/if}

		{#if showUnavailable}
			<div class="w-full">
				<Alert
					message={unavailableMessage}
					tone="error"
					icon="bi-shield-exclamation"
					onClose={undefined}
				/>
			</div>
		{:else}
			{@render children?.()}
		{/if}
	</Card>
</section>
