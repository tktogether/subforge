<script setup lang="ts">
import { endpoints } from '../api/endpoints'
import ResourcePage from '../components/ResourcePage.vue'

const resource: any = {
  list: endpoints.listSelfNodes,
  create: endpoints.createSelfNode,
  update: (item: any) => endpoints.updateSelfNode(item.id, item),
  remove: (item: any) => endpoints.deleteSelfNode(item.id),
}
</script>

<template>
  <ResourcePage
    title="自建节点"
    subtitle="自建代理节点"
    :columns="[{ key: 'link', title: '链接' }, { key: 'name', title: '名称' }, { key: 'enabled', title: '启用' }]"
    :fields="[{ key: 'link', label: '链接', type: 'text' }, { key: 'name', label: '名称', type: 'text' }, { key: 'enabled', label: '启用', type: 'switch' }]"
    :resource="resource"
  />
</template>