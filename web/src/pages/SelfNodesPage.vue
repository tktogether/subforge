<script setup lang="ts">
import { pageHelp } from '../content/help'
import ResourcePage from '../components/ResourcePage.vue'
import { endpoints } from '../api/endpoints'

const help = pageHelp('self-nodes')

const selfNodeColumns = [
  { key: 'name', title: '名称' },
  { key: 'link', title: '链接' },
  { key: 'enabled', title: '启用' },
]

const selfNodeFields = [
  { key: 'name', label: '名称', type: 'text', hint: '输出到客户端显示的名字；留空则用链接里自带的名字', example: '自建-香港' },
  { key: 'link', label: '分享链接', type: 'text', hint: '标准分享链接，以 ss:// / vmess:// / vless:// / trojan:// / hysteria2:// 开头', example: 'vless://uuid@1.2.3.4:443?encryption=none&security=tls#香港' },
  { key: 'enabled', label: '启用', type: 'switch', hint: '关闭后不参与输出' },
]

const resource: any = {
  list: endpoints.listSelfNodes,
  create: endpoints.createSelfNode,
  update: (item: any) => endpoints.updateSelfNode(item.id, item),
  remove: (item: any) => endpoints.deleteSelfNode(item.id),
}
</script>

<template>
  <div class="self-nodes-page">
    <p class="text-body-2 text-medium-emphasis mb-3">
      自建节点与机场节点合并输出。分享链接需为标准格式，保存时会校验名称、链接与必要字段。
    </p>

    <ResourcePage
      title="自建节点"
      :subtitle="help.oneLiner"
      help-key="self-nodes"
      :columns="selfNodeColumns"
      :fields="selfNodeFields"
      :resource="resource"
      :empty="help.emptyState"
    />

    <v-expansion-panels class="mt-4" data-testid="self-node-formats">
      <v-expansion-panel>
        <v-expansion-panel-title>查看格式示例</v-expansion-panel-title>
        <v-expansion-panel-text>
          <ul class="text-body-2">
            <li><code>ss://base64(method:password)@server:port#名称</code></li>
            <li><code>vmess://base64({...})</code></li>
            <li><code>vless://uuid@server:port?encryption=none&security=tls#名称</code></li>
            <li><code>trojan://password@server:port?sni=example.com#名称</code></li>
          </ul>
          <p class="text-caption text-medium-emphasis mb-0">以上均为示例，请替换成自己的服务器信息。</p>
        </v-expansion-panel-text>
      </v-expansion-panel>
    </v-expansion-panels>
  </div>
</template>
