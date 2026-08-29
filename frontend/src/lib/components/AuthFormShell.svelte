<script lang="ts">
	import Alert from "$lib/components/Alert.svelte";
	import Button from "$lib/components/Button.svelte";
	import Card from "$lib/components/Card.svelte";
	import { getContext } from "svelte";
	import { LOCALIZATION_CONTEXT, type LocalizationRuntime } from "$lib/localization/runtime";

	const localization = getContext<LocalizationRuntime>(LOCALIZATION_CONTEXT);

	interface Props {
		title: string;
		oidcEnabled?: boolean;
		oidcDescription?: string;
		unavailableMessage?: string;
		showUnavailable?: boolean;
		onOidc?: () => void;
		children?: import("svelte").Snippet;
		alerts?: import("svelte").Snippet;
	}

	let {
		title,
		oidcEnabled = false,
		oidcDescription = "",
		unavailableMessage = "",
		showUnavailable = false,
		onOidc,
		children,
		alerts,
	}: Props = $props();
</script>

<div class="mt-[10%] flex flex-col items-center justify-center">
	<h1 class="text-4xl font-bold">{title}</h1>
	{@render alerts?.()}
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
				onclick={onOidc}
			>
				<i class="bi bi-shield-lock" aria-hidden="true"></i>
				<span
					>{$localization.translate(
						"storefront.account.continue_with_provider",
						"Continue with OpenID Connect"
					)}</span
				>
			</Button>
			{#if oidcDescription}
				<p class="w-full text-center text-sm text-gray-600 dark:text-gray-300">
					{oidcDescription}
				</p>
			{/if}
			<div
				class="flex w-full items-center gap-3 py-1 text-xs tracking-[0.3em] text-gray-500 uppercase"
			>
				<div class="h-px flex-1 bg-gray-300 dark:bg-gray-700"></div>
				<span>{$localization.translate("storefront.account.or", "Or")}</span>
				<div class="h-px flex-1 bg-gray-300 dark:bg-gray-700"></div>
			</div>
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
</div>
