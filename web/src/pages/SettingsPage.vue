<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { endpoints, type SettingItem } from '../api/endpoints'
import { errorMessage } from '../composables/useAsync'
import { notify } from '../composables/useNotify'
import { fieldHelp } from '../content/help'
import SfPageHead from '../components/ui/SfPageHead.vue'
import SfSwitch from '../components/ui/SfSwitch.vue'
import SfInfoTip from '../components/ui/SfInfoTip.vue'
import SfNote from '../components/ui/SfNote.vue'
import { IconRefresh, IconDeviceFloppy, IconArrowBackUp } from '@tabler/icons-vue'

const error = ref('')
const loading = ref(false)
const settings = ref<SettingItem[]>([])
const saving = ref(false)

// 转换模板（INI）：只有启动时指定 -config 才存在；保存后若路径就是 -config，服务会热重载。
const templatePath = ref('')
const templateContent = ref('')
const templateAvailable = ref(false)
const templateLoading = ref(false)
const templateSaving = ref(false)
const templateError = ref('')

async function loadTemplate(): Promise<void> {
  templateLoading.value = true
  templateError.value = ''
  try {
    const r = await endpoints.getTemplate()
    templatePath.value = r.path
    templateContent.value = r.content
    templateAvailable.value = true
  } catch (err) {
    templateAvailable.value = false
    templatePath.value = ''
    templateContent.value = ''
    templateError.value = errorMessage(err)
  } finally {
    templateLoading.value = false
  }
}

async function saveTemplate(): Promise<void> {
  templateSaving.value = true
  templateError.value = ''
  try {
    await endpoints.putTemplate(templateContent.value)
    notify('模板已保存', 'ok')
    await loadTemplate()
  } catch (err) {
    templateError.value = errorMessage(err)
    notify(errorMessage(err), 'error')
  } finally {
    templateSaving.value = false
  }
}

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const st = await endpoints.listSettings()
    settings.value = st.settings ?? []
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

function setSwitch(item: SettingItem, value: boolean): void {
  item.value = value ? 'true' : 'false'
}

// 数字输入框在部分浏览器会把值转成 number；统一回字符串，后端设置项只接受字符串。
function setNumber(item: SettingItem, value: unknown): void {
  item.value = value == null ? '' : String(value)
}

function resetDefaults(): void {
  for (const it of settings.value) {
    it.value = ''
  }
}

async function save(): Promise<void> {
  saving.value = true
  error.value = ''
  try {
    const payload: Record<string, string> = {}
    for (const it of settings.value) {
      payload[it.key] = it.value
    }
    const r = await endpoints.updateSettings(payload)
    settings.value = r.settings ?? (await endpoints.listSettings()).settings ?? []
    notify('已保存', 'ok')
  } catch (err) {
    error.value = errorMessage(err)
    notify(errorMessage(err), 'error')
  } finally {
    saving.value = false
  }
}

const settingCount = computed(() => settings.value.length)

onMounted(() => {
  void load()
  void loadTemplate()
})
</script>

<template>
  <div>
    <SfPageHead>
      <template #actions>
        <button class="sf-btn sf-btn--subtle" type="button" data-testid="settings-reset" @click="resetDefaults">
          <IconArrowBackUp :size="15" :stroke-width="1.8" />
          重置为默认
        </button>
        <button class="sf-btn sf-btn--primary" type="button" data-testid="settings-save" :disabled="saving" @click="save">
          <IconDeviceFloppy :size="15" :stroke-width="1.8" />
          {{ saving ? '保存中…' : '保存设置' }}
        </button>
      </template>
    </SfPageHead>

    <section class="sf-panel sf-rise sf-d1" style="margin-bottom: 16px">
      <div class="sf-panel__head">
        <span class="sf-label-mono">RUNTIME / 运行参数</span>
        <span class="sf-spacer" />
        <span class="sf-faint" style="font-size: 12px">{{ settingCount }} 项 · 清空输入框表示恢复默认值</span>
      </div>

      <div v-if="loading" style="padding: 18px; display: flex; flex-direction: column; gap: 10px">
        <span v-for="n in 4" :key="n" class="sf-skeleton" :style="{ width: 40 + n * 12 + '%' }" />
      </div>

      <div v-else-if="settings.length === 0 && error === ''">
        <SfNote tone="info" style="margin: 16px">当前没有可配置项。</SfNote>
      </div>

      <template v-else>
        <div v-for="item in settings" :key="item.key" data-testid="setting-row" class="set-row">
          <div class="set-row__label">
            <div class="set-row__name">
              <span data-testid="setting-label">{{ item.label }}</span>
              <SfInfoTip v-if="(fieldHelp(item.key)?.hint ?? '') !== ''" :text="fieldHelp(item.key)?.hint ?? ''" />
            </div>
            <div data-testid="setting-desc" class="set-row__desc">{{ item.description }}</div>
            <code data-testid="setting-key" class="set-row__key">{{ item.key }}</code>
          </div>
          <div class="set-row__control">
            <SfSwitch
              v-if="item.kind === 'bool'"
              data-testid="setting-switch"
              :model-value="item.value === 'true'"
              @update:model-value="setSwitch(item, $event)"
            />
            <span v-else-if="item.kind === 'select'" class="sf-select-wrap" style="max-width: 320px">
              <select
                data-testid="setting-select"
                class="sf-select"
                :value="item.value"
                @change="item.value = ($event.target as HTMLSelectElement).value"
              >
                <option v-for="opt in item.options ?? []" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
              </select>
            </span>
            <input
              v-else-if="item.kind === 'number'"
              data-testid="setting-number"
              class="sf-input sf-table__num"
              style="max-width: 200px"
              type="number"
              min="0"
              :value="item.value"
              :placeholder="item.placeholder"
              @input="setNumber(item, ($event.target as HTMLInputElement).value)"
            />
            <input
              v-else
              v-model="item.value"
              data-testid="setting-input"
              class="sf-input"
              style="max-width: 360px"
              :placeholder="item.placeholder"
              spellcheck="false"
            />
            <div v-if="item.is_default === true" data-testid="setting-default-badge" class="set-row__meta">当前为默认值</div>
            <div v-if="item.placeholder" data-testid="setting-placeholder" class="set-row__meta">默认：{{ item.placeholder }}</div>
          </div>
        </div>
      </template>
    </section>

    <section class="sf-panel sf-rise sf-d2" data-testid="template-card">
      <div class="sf-panel__head">
        <span class="sf-panel__title">转换模板（INI）</span>
        <span class="sf-spacer" />
        <button class="sf-btn sf-btn--subtle sf-btn--sm" type="button" data-testid="template-reload" :disabled="templateLoading" @click="loadTemplate">
          <IconRefresh :size="14" :stroke-width="1.8" :class="{ 'set-spin': templateLoading }" />
          重新加载
        </button>
      </div>
      <div class="sf-panel__body">
        <div v-if="templateLoading" style="display: flex; flex-direction: column; gap: 10px">
          <span v-for="n in 5" :key="n" class="sf-skeleton" :style="{ width: 30 + n * 13 + '%' }" />
        </div>
        <template v-else-if="templateAvailable">
          <p data-testid="template-path" class="sf-field__hint" style="margin: 0 0 10px">路径：{{ templatePath }}</p>
          <textarea
            v-model="templateContent"
            data-testid="template-content"
            class="sf-textarea set-template-editor"
            rows="16"
            spellcheck="false"
          ></textarea>
          <div style="display: flex; align-items: center; gap: 12px; margin-top: 12px; flex-wrap: wrap">
            <button class="sf-btn sf-btn--primary" type="button" data-testid="template-save" :disabled="templateSaving" @click="saveTemplate">
              {{ templateSaving ? '保存中…' : '保存模板' }}
            </button>
            <span class="sf-field__hint">保存后若该路径就是启动时的 -config，服务会在约 1 秒内热重载。</span>
          </div>
        </template>
        <SfNote v-else tone="info" data-testid="template-unavailable">
          当前未启用转换模板（启动时未指定 -config）。如需自定义模板，请使用 <code>-config /data/template.ini</code> 指定模板文件后重新启动。
        </SfNote>
        <div v-if="templateError !== ''" data-testid="template-error" class="sf-field__error" style="margin-top: 10px">{{ templateError }}</div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.set-row {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) minmax(220px, 380px);
  gap: 20px;
  align-items: start;
  padding: 14px 18px;
  border-bottom: 1px solid oklch(from var(--sf-border) l c h / 0.55);
}

.set-row:last-of-type {
  border-bottom: 0;
}

.set-row__name {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  font-size: 13.5px;
}

.set-row__desc {
  margin-top: 2px;
  font-size: 12.5px;
  line-height: 1.6;
  color: var(--sf-text-muted);
}

.set-row__key {
  display: inline-block;
  margin-top: 5px;
  color: var(--sf-text-faint);
  font-family: var(--sf-font-mono);
  font-size: 10.5px;
  letter-spacing: 0.03em;
}

.set-row__control {
  min-width: 0;
}

.set-row__meta {
  margin-top: 6px;
  font-size: 12px;
  color: var(--sf-text-faint);
}

@media (max-width: 700px) {
  .set-row {
    grid-template-columns: 1fr;
    gap: 8px;
  }
}

.set-template-editor {
  font-family: var(--sf-font-code);
  font-size: 12px;
  line-height: 1.6;
  min-height: 360px;
}

.set-spin {
  animation: set-rotate 0.9s linear infinite;
}

@keyframes set-rotate {
  to {
    transform: rotate(360deg);
  }
}
</style>
