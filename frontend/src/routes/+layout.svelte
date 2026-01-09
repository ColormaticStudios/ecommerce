<script lang="ts">
	import "./main.css";
	import "bootstrap-icons/font/bootstrap-icons.css";
	import { API } from "$lib/api";
	import { userStore } from "$lib/user";
	import { onMount, setContext } from "svelte";
	import { resolve } from "$app/paths";

	const api = new API();
	setContext("api", api);

	onMount(async () => {
		api.tokenFromCookie();

		if (api.isAuthenticated()) {
			userStore.load(api);
		}
	});

	interface Props {
		children?: import("svelte").Snippet;
	}
	let { children }: Props = $props();
</script>

<svelte:head>
	<!-- <link rel="icon" href="" /> -->
</svelte:head>

<nav class="flex items-center justify-between bg-gray-100 px-3 py-2 dark:bg-gray-900">
	<div>
		<a href={resolve("/")} class="navlink text-2xl">Home</a>
	</div>
	{#if $userStore}
		{$userStore.name}
	{:else}
		<div>
			<a href={resolve("/login")} class="navlink text-xl">Log In</a>
			<a href={resolve("/signup")} class="navlink text-xl">Sign Up</a>
		</div>
	{/if}
</nav>

{@render children?.()}

<style>
	@reference "tailwindcss";

	a.navlink {
		@apply px-2 dark:text-white;
		@apply hover:text-gray-500 dark:hover:text-gray-300;
		@apply transition-[color] duration-200;
	}
</style>
