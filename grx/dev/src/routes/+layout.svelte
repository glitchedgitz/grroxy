<script lang="ts">
	import '$lib/app.css';
	let { children } = $props();
</script>

<svelte:head>
	<title>Grroxy API Tester</title>
</svelte:head>

<div class="min-h-screen bg-dark text-ivory">
	{@render children()}
</div>
