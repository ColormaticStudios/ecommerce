<script lang="ts">
	import { type API } from "$lib/api";
	import Alert from "$lib/components/Alert.svelte";
	import Button from "$lib/components/Button.svelte";
	import ButtonLink from "$lib/components/ButtonLink.svelte";
	import Card from "$lib/components/Card.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import { formatPrice } from "$lib/utils";
	import { userStore } from "$lib/user";
	import { resolve } from "$app/paths";
	import { getContext } from "svelte";
	import { SvelteURLSearchParams } from "svelte/reactivity";
	import type { PageData } from "./$types";
	import type { OrderModel } from "$lib/models";

	const api: API = getContext("api");

	interface Props {
		data: PageData;
	}

	let { data }: Props = $props();

	let email = $state("");
	let token = $state("");
	let errorMessage = $state("");
	let successMessage = $state("");
	let claiming = $state(false);
	let claimedOrder = $state<OrderModel | null>(null);

	const currency = $derived($userStore?.currency ?? "USD");
	const loginHref = $derived(
		`${resolve("/login")}?redirect=${encodeURIComponent(
			`/orders/claim${buildClaimQuery(email, token)}`
		)}`
	);
	const signupHref = $derived(
		`${resolve("/signup")}?redirect=${encodeURIComponent(
			`/orders/claim${buildClaimQuery(email, token)}`
		)}`
	);

	function buildClaimQuery(emailValue: string, tokenValue: string): string {
		const params = new SvelteURLSearchParams();
		if (emailValue.trim()) {
			params.set("email", emailValue.trim());
		}
		if (tokenValue.trim()) {
			params.set("token", tokenValue.trim());
		}
		const query = params.toString();
		return query ? `?${query}` : "";
	}

	function getOrderSummary(order: OrderModel): string {
		const itemCount = order.items.reduce((total, item) => total + item.quantity, 0);
		return `${itemCount} item${itemCount === 1 ? "" : "s"} · ${formatPrice(order.total, currency)}`;
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (claiming || !data.isAuthenticated) {
			return;
		}

		errorMessage = "";
		successMessage = "";
		claimedOrder = null;

		const trimmedEmail = email.trim();
		const trimmedToken = token.trim();
		if (!trimmedEmail || !trimmedToken) {
			errorMessage = "Enter the email and confirmation token from the guest order.";
			return;
		}

		claiming = true;
		try {
			const result = await api.claimGuestOrder({
				email: trimmedEmail,
				confirmation_token: trimmedToken,
			});
			claimedOrder = result.order;
			successMessage = result.message || "Order claimed.";
		} catch (err) {
			const error = err as { status?: number; body?: { error?: string; code?: string } };
			if (error.status === 409 || error.body?.code === "order_already_claimed") {
				errorMessage = "This guest order has already been claimed.";
			} else if (error.status === 404) {
				errorMessage = "No guest order matched that email and confirmation token.";
			} else {
				errorMessage = error.body?.error ?? "Unable to claim this guest order.";
			}
			console.error(err);
		} finally {
			claiming = false;
		}
	}

	$effect(() => {
		email = data.initialEmail;
		token = data.initialToken;
	});
</script>

<section class="mx-auto max-w-3xl px-4 py-10">
	<div class="mb-6">
		<a
			href={resolve("/orders")}
			class="inline-flex items-center gap-2 text-sm font-medium text-gray-600 transition hover:text-gray-900 dark:text-gray-300 dark:hover:text-gray-100"
		>
			<i class="bi bi-arrow-left" aria-hidden="true"></i>
			Back to orders
		</a>
	</div>

	<Card padding="xl" class="space-y-6">
		<div class="space-y-2">
			<h1 class="text-3xl font-semibold text-gray-900 dark:text-gray-100">Claim a Guest Order</h1>
			<p class="text-sm leading-6 text-gray-600 dark:text-gray-300">
				Link a past guest checkout to your account using the email and confirmation token from the
				order confirmation.
			</p>
		</div>

		{#if !data.isAuthenticated}
			<Alert
				message="Sign in or create an account before claiming a guest order."
				tone="error"
				icon="bi-shield-lock-fill"
				onClose={undefined}
			/>
			<div class="flex flex-col gap-3 sm:flex-row">
				<ButtonLink href={loginHref} variant="primary" class="text-center">Log In</ButtonLink>
				<ButtonLink href={signupHref} class="text-center">Create Account</ButtonLink>
			</div>
		{:else}
			<form class="space-y-4" onsubmit={submit}>
				<div class="grid gap-4 sm:grid-cols-2">
					<label class="block space-y-2">
						<span class="text-sm font-medium text-gray-700 dark:text-gray-200">Email</span>
						<TextInput
							bind:value={email}
							type="email"
							name="email"
							placeholder="you@example.com"
							autocomplete="email"
							required
						/>
					</label>
					<label class="block space-y-2">
						<span class="text-sm font-medium text-gray-700 dark:text-gray-200">
							Confirmation token
						</span>
						<TextInput
							bind:value={token}
							type="text"
							name="confirmation_token"
							placeholder="Order token"
							autocomplete="off"
							required
						/>
					</label>
				</div>

				<Button variant="primary" type="submit" disabled={claiming}>
					{claiming ? "Claiming..." : "Claim Order"}
				</Button>
			</form>
		{/if}

		{#if errorMessage}
			<Alert
				message={errorMessage}
				tone="error"
				icon="bi-x-circle-fill"
				onClose={() => (errorMessage = "")}
			/>
		{/if}

		{#if successMessage && claimedOrder}
			<div
				class="rounded-lg border border-emerald-200 bg-emerald-50 p-4 dark:border-emerald-900/70 dark:bg-emerald-950/30"
			>
				<p class="font-medium text-emerald-950 dark:text-emerald-50">{successMessage}</p>
				<p class="mt-1 text-sm text-emerald-900/80 dark:text-emerald-100/80">
					Order #{claimedOrder.id} · {getOrderSummary(claimedOrder)}
				</p>
				<div class="mt-4">
					<ButtonLink href={resolve(`/orders/${claimedOrder.id}`)} variant="primary">
						View Order
					</ButtonLink>
				</div>
			</div>
		{/if}
	</Card>
</section>
