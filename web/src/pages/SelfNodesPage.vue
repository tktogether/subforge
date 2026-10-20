<script setup lang="ts">
import { pageHelp } from '../content/help'
import ResourcePage from '../components/ResourcePage.vue'
import { endpoints } from '../api/endpoints'

const help = pageHelp('self-nodes')

const selfNodeColumns = [
  { key: 'name', title: '名称' },
  { key: 'link', title: '链接' },
  { key: 'enabled', title: '启用' },
  { key: 'sort_order', title: '顺序' },
]

const selfNodeFields = [
  { key: 'name', label: '名称', type: 'text', hint: '输出到客户端显示的名字；留空则用链接里自带的名字', example: '自建-香港' },
  { key: 'link', label: '分享链接', type: 'text', hint: '标准分享链接，以 ss:// / vmess:// / vless:// / trojan:// / hysteria2:// 开头', example: 'vless://uuid@1.2.3.4:443?encryption=none&security=tls#香港' },
  { key: 'enabled', label: '启用', type: 'switch', hint: '关闭后不参与输出' },
  { key: 'sort_order', label: '排序', type: 'number', hint: '越小越靠前' },
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
    <v-alert type="info" variant="tonal" class="mb-4" data-testid="self-node-help">
      <div class="text-body-2">
        <p class="mb-1"><strong>和机场节点的区别（最关键）：</strong></p>
        <ul class="mb-2">
          <li><strong>机场节点</strong>：来自「机场」页配置的订阅地址，整份订阅一起拉取、一起更新。</li>
          <li><strong>自建节点</strong>：这里一行一条，<strong>直接填节点分享链接</strong>（<code>ss://</code>、<code>vmess://</code>、<code>vless://</code>、<code>trojan://</code>、<code>hysteria2://</code> 等）。服务会把它写成临时文件交给同一套解析引擎，<strong>和机场节点走完全相同的去重、管线、分组、规则流程</strong>。</li>
          <li>两者的节点在输出里<strong>没有区别</strong>，只是来源不同。</li>
        </ul>
        <p class="mb-1"><strong>什么时候用它：</strong>机场里没有的机器（自己的 VPS）、临时补一个节点、或者不想把链接放进机场订阅时。</p>
        <p class="mb-0"><strong>链接格式：</strong>必须是<strong>标准分享链接</strong>（客户端里「复制链接」得到的那一串）。保存时服务端会逐条解析校验：名称或链接为空、链接解析失败、节点字段校验不通过，都会<strong>直接返回 400 并报错</strong>（提示形如「分享链接格式不正确：…」），<strong>不会静默入列表</strong>——加完可以去「概览」页核对节点数。</p>
      </div>
    </v-alert>

    <ResourcePage
      title="自建节点"
      subtitle="手动添加的节点，与机场节点合并输出"
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
