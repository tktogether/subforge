<script setup lang="ts" generic="T extends Record<string, unknown>">
import SfEmpty from './SfEmpty.vue'
import { IconInbox } from '@tabler/icons-vue'

export interface SfColumn {
  key: string
  label: string
  width?: string
  align?: 'left' | 'right' | 'center'
  mono?: boolean
}

const props = withDefaults(
  defineProps<{
    columns: SfColumn[]
    rows: T[]
    rowKey: (row: T) => string | number
    loading?: boolean
    clickable?: boolean
    emptyTitle?: string
    emptyDesc?: string
  }>(),
  { loading: false, clickable: false, emptyTitle: '暂无数据', emptyDesc: '' },
)

const emit = defineEmits<{ (e: 'row-click', row: T): void }>()

function cellValue(row: T, key: string): unknown {
  return row[key]
}

function skeletonCount(): number {
  return Math.min(Math.max(props.rows.length, 3), 8)
}
</script>

<template>
  <div class="sf-table-wrap">
    <table class="sf-table">
      <thead>
        <tr>
          <th
            v-for="col in columns"
            :key="col.key"
            :style="{ width: col.width, textAlign: col.align }"
          >
            {{ col.label }}
          </th>
        </tr>
      </thead>
      <tbody>
        <template v-if="loading">
          <tr v-for="n in skeletonCount()" :key="'skeleton-' + n">
            <td v-for="col in columns" :key="col.key">
              <span class="sf-skeleton" :style="{ width: 40 + ((n * 17 + col.key.length * 13) % 50) + '%' }" />
            </td>
          </tr>
        </template>
        <tr v-else-if="rows.length === 0">
          <td :colspan="columns.length" style="padding: 0">
            <slot name="empty">
              <SfEmpty :title="emptyTitle" :desc="emptyDesc">
                <template #icon><IconInbox :size="30" :stroke-width="1.4" /></template>
              </SfEmpty>
            </slot>
          </td>
        </tr>
        <template v-else>
          <tr
            v-for="row in rows"
            :key="rowKey(row)"
            :class="{ 'is-clickable': clickable }"
            @click="clickable ? emit('row-click', row) : undefined"
          >
            <td
              v-for="col in columns"
              :key="col.key"
              :class="{ 'sf-table__num': col.mono }"
              :style="{ textAlign: col.align }"
            >
              <slot :name="'cell-' + col.key" :row="row">{{ cellValue(row, col.key) }}</slot>
            </td>
          </tr>
        </template>
      </tbody>
    </table>
  </div>
</template>
