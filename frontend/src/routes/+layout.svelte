<script lang="ts">
	import "./main.css";
	import "bootstrap-icons/font/bootstrap-icons.css";
	import { API } from "$lib/api";
	import { userStore } from "$lib/user";
	import { onMount, setContext } from "svelte";
	import type { LayoutData } from "./$types";

	interface Props {
		data: LayoutData;
		children?: import("svelte").Snippet;
	}

	let { data, children }: Props = $props();

	const api = new API();
	setContext("api", api);

	onMount(() => {
		api.bootstrapAuthState(Boolean(data.isAuthenticated));
		void userStore.load(api);
	});
</script>

{@render children?.()}
