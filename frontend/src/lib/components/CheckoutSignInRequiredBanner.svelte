<script lang="ts">
	import { resolve } from "$app/paths";
	import ButtonLink from "$lib/components/ButtonLink.svelte";
	import Card from "$lib/components/Card.svelte";
</script>

<Card tone="amber" radius="3xl" padding="lg" class="mt-6 text-gray-700 dark:text-amber-50">
	<p class="text-2xl font-semibold">You need to sign in to check out.</p>
	<p class="mt-2 max-w-2xl text-sm text-gray-600 dark:text-amber-100/80">
		Sign in or create an account to continue with this order.
	</p>
	<div class="mt-5 flex flex-wrap gap-3">
		<ButtonLink href={resolve("/login")} variant="primary" size="large">Log in</ButtonLink>
		<ButtonLink href={resolve("/signup/")} variant="regular" size="large">Create account</ButtonLink
		>
	</div>
</Card>
