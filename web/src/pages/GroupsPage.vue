<script setup lang="ts">
import { endpoints } from '../api/endpoints'
import ResourcePage from '../components/ResourcePage.vue'

const resource: any = {
  list: endpoints.listGroups,
  create: endpoints.createGroup,
  update: (item: any) => endpoints.updateGroup(item.id, item),
  remove: (item: any) => endpoints.deleteGroup(item.id),
}
</script>

<template>
  <ResourcePage
    title="分组"
    subtitle="代理分组"
    :columns="[{ key: 'name', title: '名称' }, { key: 'type', title: '类型' }, { key: 'regex', title: '正则' }, { key: 'url', title: 'URL' }, { key: 'interval', title: '间隔' }]"
    :fields="[{ key: 'name', label: '名称', type: 'text' }, { key: 'type', label: '类型', type: 'text' }, { key: 'regex', label: '正则', type: 'text' }, { key: 'url', label: 'URL', type: 'text' }, { key: 'interval', label: '间隔', type: 'number' }]"
    :resource="resource"
  />
</template>