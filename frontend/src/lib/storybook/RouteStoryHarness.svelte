<script lang="ts">
	import type { API } from "$lib/api";
	import type { UserModel } from "$lib/models";
	import { User, userStore } from "$lib/user";
	import { onDestroy, setContext, untrack, type Component } from "svelte";

	interface Props {
		component: unknown;
		componentProps?: Record<string, unknown>;
		api?: Partial<API>;
		user?: UserModel | null;
	}

	let { component, componentProps = {}, api = {}, user = null }: Props = $props();

	const apiContext = untrack(() => api as API);
	const StoryComponent = untrack(() => component as Component<Record<string, never>>);
	setContext("api", apiContext);

	function createStoryUser(nextUser: UserModel): User {
		return new User(
			apiContext,
			nextUser.id,
			nextUser.subject,
			nextUser.username,
			nextUser.email,
			nextUser.name,
			nextUser.role,
			nextUser.currency,
			nextUser.profile_photo_url,
			nextUser.created_at,
			nextUser.updated_at,
			nextUser.deleted_at
		);
	}

	$effect(() => {
		if (user) {
			userStore.setUser(createStoryUser(user));
			return;
		}
		userStore.logout();
	});

	onDestroy(() => {
		userStore.logout();
	});
</script>

<StoryComponent {...(componentProps as Record<string, never>)} />
