<script setup lang="ts">
import { computed, ref } from 'vue'
import type { PageHelp } from '../content/help'

const props = defineProps<{ help: PageHelp; dense?: boolean; label?: string }>()

const open = ref(false)

const hasSteps = computed(() => Array.isArray(props.help.steps) && props.help.steps.length > 0)
const hasConcepts = computed(() => Array.isArray(props.help.concepts) && props.help.concepts.length > 0)
const hasFaq = computed(() => Array.isArray(props.help.faq) && props.help.faq.length > 0)
const hasLinks = computed(() => Array.isArray(props.help.links) && props.help.links.length > 0)
const toggleLabel = computed(() => props.label ?? props.help.oneLiner)

const hasBody = computed(
  () =>
    hasSteps.value ||
    hasConcepts.value ||
    hasFaq.value ||
    hasLinks.value ||
    (props.help.whenToUse !== '' && !props.dense),
)

function toggle(): void {
  open.value = true
}
</script>

<template>
  <div data-testid="help-panel" class="mb-3">
    <div v-if="!hasBody" data-testid="help-oneliner" class="help-static">
      {{ props.help.oneLiner }}
    </div>
    <v-btn
      v-else
      variant="text"
      size="small"
      color="primary"
      prepend-icon="mdi-help-circle-outline"
      data-testid="help-toggle"
      :aria-expanded="open"
      @click="toggle"
    >
      <span data-testid="help-toggle-label">{{ toggleLabel }}</span>
    </v-btn>

    <v-dialog v-model="open" max-width="760" scrollable data-testid="help-dialog">
      <v-card>
        <v-card-title>{{ props.help.title }} · 使用说明</v-card-title>
        <v-card-text data-testid="help-body">
          <p v-if="props.help.whenToUse !== '' && !props.dense" data-testid="help-when" class="text-body-2 text-medium-emphasis mb-3">
            {{ props.help.whenToUse }}
          </p>

          <template v-if="hasSteps">
            <div class="text-subtitle-2 mb-1">操作步骤</div>
            <ol data-testid="help-steps" class="help-list mb-4">
              <li v-for="(step, i) in props.help.steps" :key="'step-' + i">
                <span class="font-weight-medium">{{ step.title }}</span>
                <span v-if="!props.dense" class="text-medium-emphasis">：{{ step.detail }}</span>
              </li>
            </ol>
          </template>

          <template v-if="hasConcepts">
            <div class="text-subtitle-2 mb-1">概念说明</div>
            <dl data-testid="help-concepts" class="help-list mb-4">
              <div v-for="(c, i) in props.help.concepts" :key="'concept-' + i" class="mb-2">
                <dt class="font-weight-medium">{{ c.term }}</dt>
                <dd v-if="!props.dense" class="text-medium-emphasis ml-0">{{ c.explain }}</dd>
              </div>
            </dl>
          </template>

          <template v-if="hasFaq">
            <div class="text-subtitle-2 mb-1">常见问题</div>
            <dl data-testid="help-faq" class="help-list mb-4">
              <div v-for="(f, i) in props.help.faq" :key="'faq-' + i" class="mb-2">
                <dt class="font-weight-medium">{{ f.q }}</dt>
                <dd v-if="!props.dense" class="text-medium-emphasis ml-0">{{ f.a }}</dd>
              </div>
            </dl>
          </template>

          <div v-if="hasLinks" data-testid="help-links" class="d-flex flex-wrap ga-3">
            <template v-for="(link, i) in props.help.links" :key="'link-' + i">
              <router-link v-if="link.to" :to="link.to" class="text-decoration-none">
                {{ link.label }}<span v-if="link.note" class="text-medium-emphasis">（{{ link.note }}）</span>
              </router-link>
            </template>
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" data-testid="help-close" @click="open = false">关闭</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<style scoped>
.help-static {
  padding: 8px 12px;
  background: var(--sf-surface);
  border: 1px solid var(--sf-border);
  border-radius: var(--sf-radius);
  color: var(--sf-text-muted);
  font-size: 13px;
  line-height: 1.4;
}
.help-list {
  padding-left: 20px;
}
.help-list dd {
  margin-left: 0;
}
</style>
