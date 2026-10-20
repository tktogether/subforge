<script setup lang="ts">
import { computed, ref, watch } from 'vue'
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

function toggleReveal(): void {
  reveal.value = !reveal.value
}

function clearDraft(): void {
  draft.value = ''
}

function submit(): void {
  saveToken(draft.value)
  emit('saved')
  open.value = false
}
</script>

<template>
  <v-dialog v-model="open" max-width="520" data-testid="token-dialog">
    <v-card>
      <v-card-title>访问令牌</v-card-title>
      <v-card-text>
        <p class="token-hint">
          服务端以 <code>-token</code> 启动时，控制台需要携带访问令牌才能读取数据，
          令牌会作为 <code>X-Token</code> 请求头发送。服务端未设置令牌时留空即可。
        </p>
        <v-alert
          v-if="tokenRejected"
          type="warning"
          variant="tonal"
          density="comfortable"
          data-testid="token-rejected"
        >
          服务端拒绝了上一次请求（401）：令牌缺失或不正确。
        </v-alert>
        <v-text-field
          v-model="draft"
          label="访问令牌"
          placeholder="与服务端 -token 的取值一致"
          :type="reveal ? 'text' : 'password'"
          :append-inner-icon="reveal ? 'mdi-eye-off' : 'mdi-eye'"
          autocomplete="off"
          data-testid="token-input"
          @click:append-inner="toggleReveal"
        ></v-text-field>
      </v-card-text>
      <v-card-actions>
        <v-btn variant="text" data-testid="token-clear" @click="clearDraft">清除</v-btn>
        <v-spacer></v-spacer>
        <v-btn variant="text" data-testid="token-cancel" @click="open = false">取消</v-btn>
        <v-btn color="primary" data-testid="token-save" @click="submit">保存</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.token-hint {
  margin: 0 0 12px;
  font-size: 13px;
  line-height: 1.7;
  color: var(--sf-text-muted, #5a6472);
}
.token-hint code {
  padding: 1px 4px;
  border-radius: 3px;
  background: var(--sf-surface-alt, rgba(127, 127, 127, 0.12));
  font-size: 12px;
}
</style>
