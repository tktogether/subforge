<script setup lang="ts">
import { computed } from 'vue'
import type { PageHelp } from '../content/help'

const props = defineProps<{ help: PageHelp; dense?: boolean }>()

const hasSteps = computed(() => Array.isArray(props.help.steps) && props.help.steps.length > 0)
const hasConcepts = computed(() => Array.isArray(props.help.concepts) && props.help.concepts.length > 0)
const hasFaq = computed(() => Array.isArray(props.help.faq) && props.help.faq.length > 0)
const hasLinks = computed(() => Array.isArray(props.help.links) && props.help.links.length > 0)
</script>

<template>
  <div data-testid="help-panel" class="mb-4">
    <div data-testid="help-oneliner" class="text-body-2 mb-1">{{ props.help.oneLiner }}</div>
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
</template>
