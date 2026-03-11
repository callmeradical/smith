<script lang="ts">
	interface Props {
		title: string;
		description: string;
		buttonText?: string;
		buttonHref?: string;
		icon?: string;
	}

	let { 
		title, 
		description, 
		buttonText, 
		buttonHref,
		icon = "📁"
	}: Props = $props();
</script>

<div class="empty-state-container">
	<div class="empty-state-icon">{icon}</div>
	<h2 class="empty-state-title">{title}</h2>
	<p class="empty-state-description">{description}</p>
	{#if buttonText && buttonHref}
		<a href={buttonHref} class="primary empty-state-cta">
			{buttonText}
		</a>
	{/if}
</div>

<style>
	.empty-state-container {
		grid-column: 1 / -1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 64px 24px;
		text-align: center;
		background: rgba(255, 255, 255, 0.02);
		border: 1px dashed var(--border);
		border-radius: var(--radius);
		margin: 20px 0;
	}

	.empty-state-icon {
		font-size: 64px;
		margin-bottom: 24px;
		filter: drop-shadow(0 0 15px rgba(6, 182, 212, 0.3));
	}

	.empty-state-title {
		font-size: 1.5rem;
		font-weight: 700;
		color: var(--text);
		margin: 0 0 12px 0;
		letter-spacing: -0.02em;
	}

	.empty-state-description {
		font-size: 1rem;
		color: var(--muted);
		max-width: 400px;
		margin: 0 0 32px 0;
		line-height: 1.5;
	}

	.empty-state-cta {
		text-decoration: none;
		padding: 12px 24px;
		font-size: 1rem;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 160px;
	}
</style>
