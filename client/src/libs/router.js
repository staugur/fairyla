/*
   Copyright 2021 Hiroshi.tao

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

import { createRouter, createWebHashHistory } from 'vue-router'
import Index from '@/views/Index.vue'
import { mutations, actions } from './store.js'
import { TitleSep, TitleSuffix } from './vars.js'

const routes = [
    {
        path: '/',
        name: 'Index',
        component: Index
    },
    {
        path: '/login',
        name: 'Login',
        component: () => import('@/views/auth/Login.vue'),
        meta: { requiresAuth: false, title: '登录' }
    },
    {
        path: '/register',
        name: 'Register',
        component: () => import('@/views/auth/Register.vue'),
        meta: { requiresAuth: false, title: '注册' }
    },
    {
        path: '/logout',
        name: 'Logout',
        redirect() {
            mutations.clearLogin()
            actions.removeConfig()
            return '/'
        },
        meta: { requiresAuth: true }
    },
    {
        path: '/ta',
        name: 'Ta',
        component: () => import('@/views/ta/Ta.vue'),
        meta: { requiresAuth: true, title: '她是' }
    },
    {
        path: '/my',
        name: 'Home',
        component: () => import('@/views/home/Home.vue'),
        children: [
            /*
            {
                path: 'profile',
                component: () => import('@/views/home/UserProfile.vue')
            },
            {
                path: 'setting',
                component: () => import('@/views/home/UserSetting.vue')
            },
            */
            {
                path: 'album',
                component: () => import('@/views/home/UserAlbum.vue')
            }
        ],
        meta: { requiresAuth: true, title: '个人中心' }
    },
    {
        path: '/album/:name',
        name: 'Album',
        component: () => import('@/views/album/AlbumFairy.vue'),
        meta: { requiresAuth: true, title: '专辑' }
    },
    {
        path: '/:pathMatch(.*)*',
        name: 'NotFound',
        component: () => import('@/views/public/NotFound.vue'),
        meta: { title: '页面未发现' }
    }
]

const router = createRouter({
    history: createWebHashHistory(),
    routes
})

router.beforeEach((to, from) => {
    if (to.meta.title) {
        document.title = to.meta.title + TitleSep + TitleSuffix
    } else {
        document.title = TitleSuffix
    }
    // 而不是去检查每条路由记录
    // to.matched.some(record => record.meta.requiresAuth)
    if (to.meta.requiresAuth === false && mutations.isLogged()) {
        return { path: '/' }
    }
    if (to.meta.requiresAuth && !mutations.isLogged()) {
        // 此路由需要授权，请检查是否已登录
        // 如果没有，则重定向到登录页面
        return {
            name: 'Login',
            query: { redirect: to.fullPath }
        }
    }
})

export default router
