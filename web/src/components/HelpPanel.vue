<script setup lang="ts">
import { computed, ref } from 'vue'
import type { PageHelp } from '../content/help'

const props = defineProps<{ help: PageHelp; dense?: boolean; label?: string }>()

const open = ref(false)

const hasSteps = computed(() => Array.isArray(props.help.steps) && props.help.steps.length > 0)
const hasConcepts = computed(() => Array.isArray(props.help.concepts) && props.help.concepts.length > 0)
const hasFaq = computed(() => Array.isArray(props.help.faq) && props.help.faq.length > 0)
const hasLinks = computed(() => Array.isArray(props.help.links) && props.help.links.length > 0)

// 入口行文案：页面副标题已经写过 oneLiner 的页面传 label（如「使用说明」），
// 这里就不再重复那句话——重复正是这轮要治的「丑」。
const toggleLabel = computed(() => props.label ?? props.help.oneLiner)

// 没有可展开的内容（未知 helpKey 的兜底文案）时不留一个点不开的按钮。
const hasBody = computed(
  () =>
    hasSteps.value ||
    hasConcepts.value ||
    hasFaq.value ||
    hasLinks.value ||
    (props.help.whenToUse !== '' && !props.dense),
)

function toggle(): void {
  open.value = !open.value
}
</script>

<template>
  <div data-testid="help-panel" class="mb-4">
    <div v-if="!hasBody" data-testid="help-oneliner" class="help-static">
      {{ props.help.oneLiner }}
    </div>

    <button
      v-else
      type="button"
      data-testid="help-toggle"
      class="help-toggle"
      :aria-expanded="open"
      @click="toggle"
    >
      <span data-testid="help-toggle-label" class="help-toggle__label">{{ toggleLabel }}</span>
      <v-icon size="small" class="help-toggle__chevron" aria-hidden="true">
        {{ open ? 'mdi-chevron-up' : 'mdi-chevron-down' }}
      </v-icon>
    </button>

    <div v-if="open && hasBody" data-testid="help-body" class="help-body">
      <div v-if="props.help.whenToUse !== '' && !props.dense" data-testid="help-when" class="text-body-2 text-medium-emphasis mb-2">
        {{ props.help.whenToUse }}
      </div>

      <v-expansion-panels v-if="hasSteps" :key="'steps'" data-testid="help-steps" variant="accordion" multiple flat>
        <v-expansion-panel v-for="(step, i) in props.help.steps" :key="'step-' + i">
          <v-expansion-panel-title :hide-on-click="!props.dense">
            {{ i + 1 }}. {{ step.title }}
          </v-expansion-panel-title>
          <v-expansion-panel-text v-if="!props.dense" eager>{{ step.detail }}</v-expansion-panel-text>
        </v-expansion-panel>
      </v-expansion-panels>

      <v-expansion-panels v-if="hasConcepts" :key="'concepts'" data-testid="help-concepts" variant="accordion" multiple flat>
        <v-expansion-panel v-for="(c, i) in props.help.concepts" :key="'concept-' + i">
          <v-expansion-panel-title :hide-on-click="!props.dense">{{ c.term }}</v-expansion-panel-title>
          <v-expansion-panel-text v-if="!props.dense" eager>{{ c.explain }}</v-expansion-panel-text>
        </v-expansion-panel>
      </v-expansion-panels>

      <v-expansion-panels v-if="hasFaq" :key="'faq'" data-testid="help-faq" variant="accordion" multiple flat>
        <v-expansion-panel v-for="(f, i) in props.help.faq" :key="'faq-' + i">
          <v-expansion-panel-title :hide-on-click="!props.dense">{{ f.q }}</v-expansion-panel-title>
          <v-expansion-panel-text v-if="!props.dense" eager>{{ f.a }}</v-expansion-panel-text>
        </v-expansion-panel>
      </v-expansion-panels>

      <div v-if="hasLinks" data-testid="help-links" class="d-flex flex-wrap ga-2 mt-2">
        <template v-for="(link, i) in props.help.links" :key="'link-' + i">
          <router-link v-if="link.to" :to="link.to" class="text-decoration-none">
            {{ link.label }}<span v-if="link.note" class="text-medium-emphasis">（{{ link.note }}）</span>
          </router-link>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.help-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  padding: 8px 12px;
  background: var(--sf-surface);
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  color: var(--sf-text);
  font-size: 13px;
  line-height: 1.4;
  cursor: pointer;
  text-align: left;
  transition: background 150ms var(--sf-ease);
}
.help-static {
  padding: 8px 12px;
  background: var(--sf-surface);
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  color: var(--sf-text-muted);
  font-size: 13px;
  line-height: 1.4;
}
.help-toggle:hover {
  background: var(--sf-surface-raised);
}
.help-toggle:focus-visible {
  outline: 2px solid var(--sf-accent);
  outline-offset: 2px;
}
.help-toggle__label {
  flex: 1 1 auto;
  min-width: 0;
}
.help-toggle__chevron {
  color: var(--sf-text-muted);
  flex: 0 0 auto;
}
.help-body {
  margin-top: 8px;
}
</style>
