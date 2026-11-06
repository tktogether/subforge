<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { IconEye, IconEyeOff } from '@tabler/icons-vue'
import SfDialog from './ui/SfDialog.vue'
import SfField from './ui/SfField.vue'
import { saveToken, tokenRejected, tokenState } from '../api/token'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; saved: [] }>()

const draft = ref('')
const reveal = ref(false)

const open = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

// 每次打开都把输入框重置成当前令牌：用户能看到已保存的内容，改错字也方便。
watch(
  () => props.modelValue,
  (visible) => {
    if (visible) {
      draft.value = tokenState.value
      reveal.value = false
    }
  },
  { immediate: true },
)

function submit(): void {
  saveToken(draft.value)
  emit('saved')
  open.value = false
}
</script>

<template>
  <SfDialog v-model:open="open" label="ACCESS TOKEN" title="访问令牌">
    <p class="token-hint">
      服务端以 <code>-token</code> 启动时，控制台需要携带访问令牌才能读取数据，
      令牌会作为 <code>X-Token</code> 请求头发送。服务端未设置令牌时留空即可。
    </p>
    <div v-if="tokenRejected" class="sf-note sf-note--warn" data-testid="token-rejected">
      <span class="sf-note__body">服务端拒绝了上一次请求（401）：令牌缺失或不正确。</span>
    </div>
    <SfField label="访问令牌">
      <div class="token-input-row">
        <input
          v-model="draft"
          class="sf-input sf-input--mono"
          :type="reveal ? 'text' : 'password'"
          placeholder="与服务端 -token 的取值一致"
          autocomplete="off"
          spellcheck="false"
          data-testid="token-input"
          @keydown.enter="submit"
        />
        <button class="sf-btn sf-btn--ghost sf-btn--icon" type="button" :aria-label="reveal ? '隐藏令牌' : '显示令牌'" @click="reveal = !reveal">
          <IconEyeOff v-if="reveal" :size="16" :stroke-width="1.7" />
          <IconEye v-else :size="16" :stroke-width="1.7" />
        </button>
      </div>
    </SfField>
    <template #foot>
      <button class="sf-btn sf-btn--subtle" type="button" data-testid="token-clear" @click="draft = ''">清除</button>
      <span class="sf-spacer" />
      <button class="sf-btn sf-btn--ghost" type="button" data-testid="token-cancel" @click="open = false">取消</button>
      <button class="sf-btn sf-btn--primary" type="button" data-testid="token-save" @click="submit">保存令牌</button>
    </template>
  </SfDialog>
</template>

<style scoped>
.token-hint {
  margin: 0;
  font-size: 13px;
  line-height: 1.75;
  color: var(--sf-text-muted);
}

.token-hint code {
  padding: 1px 5px;
  border-radius: 3px;
  background: var(--sf-surface-dim);
  border: 1px solid var(--sf-border);
  font-size: 12px;
}

.token-input-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
</style>
