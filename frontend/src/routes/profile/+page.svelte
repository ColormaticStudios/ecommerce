<script lang="ts">
	import { type API } from "$lib/api";
	import { getProfile, userStore } from "$lib/user";
	import { getContext, onDestroy, onMount } from "svelte";
	import { resolve } from "$app/paths";

	const api: API = getContext("api");

	let loading = $state(true);
	let errorMessage = $state("");
	let statusMessage = $state("");
	let name = $state("");
	let currency = $state("USD");
	let email = $state("");
	let username = $state("");
	let profilePhotoUrl = $state<string | null>(null);
	let selectedFile = $state<File | null>(null);
	let previewUrl = $state<string | null>(null);
	let uploading = $state(false);
	let removing = $state(false);
	let authChecked = $state(false);

	function clearPreview() {
		if (previewUrl) {
			URL.revokeObjectURL(previewUrl);
		}
		previewUrl = null;
		selectedFile = null;
	}

	function handleFileChange(event: Event) {
		const target = event.target as HTMLInputElement;
		const file = target.files?.[0];
		if (!file) {
			clearPreview();
			return;
		}
		clearPreview();
		selectedFile = file;
		previewUrl = URL.createObjectURL(file);
	}

	async function loadProfile() {
		api.tokenFromCookie();
		authChecked = true;
		if (!api.isAuthenticated()) {
			loading = false;
			return;
		}

		loading = true;
		errorMessage = "";
		statusMessage = "";

		const user = await getProfile(api);
		if (!user) {
			errorMessage = "Unable to load your profile. Please log in again.";
			loading = false;
			return;
		}

		userStore.setUser(user);
		name = user.name ?? "";
		currency = user.currency ?? "USD";
		email = user.email;
		username = user.username;
		profilePhotoUrl = user.profile_photo_url;
		loading = false;
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		statusMessage = "";
		errorMessage = "";

		try {
			await api.updateProfile({
				name: name.trim() || undefined,
				currency: currency.trim() || undefined,
			});
			await loadProfile();
			statusMessage = "Profile updated.";
		} catch (err) {
			console.error(err);
			errorMessage = "Could not update profile. Please try again.";
		}
	}

	async function uploadPhoto() {
		if (!selectedFile) {
			return;
		}

		api.tokenFromCookie();
		uploading = true;
		errorMessage = "";
		statusMessage = "";

		try {
			const mediaId = await api.uploadMedia(selectedFile);
			await api.attachProfilePhoto(mediaId);
			await loadProfile();
			statusMessage = "Profile photo updated.";
			clearPreview();
		} catch (err) {
			console.error(err);
			const error = err as { status?: number; body?: { error?: string } };
			if (error.status === 409 && error.body?.error === "Media is still processing") {
				errorMessage = "Photo is still processing. Please try again in a moment.";
			} else if (error.status === 422 && error.body?.error) {
				errorMessage = error.body.error;
			} else {
				errorMessage = error.body?.error ?? "Could not upload the photo. Please try again.";
			}
		} finally {
			uploading = false;
		}
	}

	async function removePhoto() {
		if (!profilePhotoUrl) {
			return;
		}

		api.tokenFromCookie();
		removing = true;
		errorMessage = "";
		statusMessage = "";

		try {
			await api.removeProfilePhoto();
			await loadProfile();
			statusMessage = "Profile photo removed.";
		} catch (err) {
			console.error(err);
			errorMessage = "Could not remove the photo.";
		} finally {
			removing = false;
		}
	}

	onMount(loadProfile);
	onDestroy(clearPreview);
</script>

<section>
	<div class="mx-auto max-w-5xl px-4 py-10">
		<h1 class="text-3xl font-semibold text-gray-900 dark:text-gray-100">Profile</h1>

		{#if !authChecked}
			<div class="mt-6 grid gap-6 md:grid-cols-[280px_1fr]">
				<div
					class="h-64 animate-pulse rounded-2xl border border-gray-200 bg-gray-100 dark:border-gray-700 dark:bg-gray-800"
				></div>
				<div
					class="h-64 animate-pulse rounded-2xl border border-gray-200 bg-gray-100 dark:border-gray-700 dark:bg-gray-800"
				></div>
			</div>
		{:else if !api.isAuthenticated()}
			<p class="mt-4 text-gray-600 dark:text-gray-300">
				Please
				<a href={resolve("/login")} class="text-blue-600 hover:underline dark:text-blue-400">
					log in
				</a>
				to view your profile.
			</p>
		{:else if loading}
			<div class="mt-6 grid gap-6 md:grid-cols-[280px_1fr]">
				<div
					class="h-64 animate-pulse rounded-2xl border border-gray-200 bg-gray-100 dark:border-gray-700 dark:bg-gray-800"
				></div>
				<div
					class="h-64 animate-pulse rounded-2xl border border-gray-200 bg-gray-100 dark:border-gray-700 dark:bg-gray-800"
				></div>
			</div>
		{:else}
			<div class="mt-8 grid gap-6 md:grid-cols-[280px_1fr]">
				<div
					class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
				>
					<div class="flex flex-col items-center text-center">
						<div
							class="h-28 w-28 overflow-hidden rounded-full border border-gray-200 bg-gray-100 shadow-sm dark:border-gray-700 dark:bg-gray-800"
						>
							{#if previewUrl}
								<img src={previewUrl} alt="Profile preview" class="h-full w-full object-cover" />
							{:else if profilePhotoUrl}
								<img src={profilePhotoUrl} alt="Profile" class="h-full w-full object-cover" />
							{:else}
								<div
									class="flex h-full w-full items-center justify-center text-2xl font-semibold text-gray-500 dark:text-gray-300"
								>
									{(name || username || "?").slice(0, 1).toUpperCase()}
								</div>
							{/if}
						</div>
						<h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-gray-100">
							{name || username}
						</h2>
						<p class="text-sm text-gray-500 dark:text-gray-400">@{username}</p>
						<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{email}</p>
					</div>

					<div class="mt-6 space-y-3 text-sm text-gray-600 dark:text-gray-300">
						<label class="btn btn-regular inline-flex w-full items-center justify-between">
							<input type="file" accept="image/*" class="hidden" onchange={handleFileChange} />
							<i class="bi bi-folder-fill"></i>
							Choose photo
							<span></span>
						</label>
						<button
							type="button"
							class="btn btn-primary w-full"
							disabled={!selectedFile || uploading}
							onclick={uploadPhoto}
						>
							<i class="bi bi-upload float-left"></i>
							{uploading ? "Uploading..." : "Upload photo"}
						</button>
						<button
							type="button"
							class="btn btn-warning w-full"
							disabled={!profilePhotoUrl || removing}
							onclick={removePhoto}
						>
							<i class="bi bi-trash-fill float-left"></i>
							{removing ? "Removing..." : "Remove photo"}
						</button>
						<p class="text-xs text-gray-500 dark:text-gray-400">
							Recommended square image, up to 5MB.
						</p>
					</div>
				</div>

				<div
					class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
				>
					<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Account details</h3>
					<form class="mt-6 space-y-4" onsubmit={submit}>
						<div class="grid gap-4 md:grid-cols-2">
							<div>
								<label for="username" class="text-sm font-medium text-gray-600 dark:text-gray-300">
									Username
								</label>
								<input id="username" class="textinput mt-1" type="text" value={username} readonly />
							</div>
							<div>
								<label for="email" class="text-sm font-medium text-gray-600 dark:text-gray-300">
									Email
								</label>
								<input id="email" class="textinput mt-1" type="email" value={email} readonly />
							</div>
						</div>

						<div class="grid gap-4 md:grid-cols-2">
							<div>
								<label for="name" class="text-sm font-medium text-gray-600 dark:text-gray-300">
									Name
								</label>
								<input
									id="name"
									class="textinput mt-1"
									type="text"
									bind:value={name}
									placeholder="Your name"
								/>
							</div>
							<div>
								<label for="currency" class="text-sm font-medium text-gray-600 dark:text-gray-300">
									Preferred currency
								</label>
								<input
									id="currency"
									class="textinput mt-1"
									type="text"
									bind:value={currency}
									placeholder="USD"
								/>
							</div>
						</div>

						{#if errorMessage}
							<p class="text-sm text-red-500">{errorMessage}</p>
						{/if}
						{#if statusMessage}
							<p class="text-sm text-green-600">{statusMessage}</p>
						{/if}

						<button class="btn btn-large btn-primary float-right" type="submit">
							<i class="bi bi-floppy-fill mr-1"></i>
							Save changes
						</button>
					</form>
				</div>
			</div>
		{/if}
	</div>
</section>
