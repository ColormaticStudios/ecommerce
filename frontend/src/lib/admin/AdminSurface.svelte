<script lang="ts">
	import { adminSurfaceVariantClasses, type AdminSurfaceVariant } from "$lib/admin/tokens";

	interface Props {
		as?: string;
		variant?: AdminSurfaceVariant;
		class?: string;
		children?: import("svelte").Snippet;
	}

	let { as = "section", variant = "panel", class: className = "", children }: Props = $props();
	const variantClasses = $derived(adminSurfaceVariantClasses[variant]);
</script>

<svelte:element this={as} class={`${variantClasses} ${className}`.trim()}>
	{@render children?.()}
</svelte:element>
