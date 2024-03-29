<template>
    <el-container>
        <el-header>
            <Navbar />
        </el-header>
        <el-main>
            <router-view />
        </el-main>
        <el-footer>
            <Footer />
        </el-footer>
    </el-container>
</template>

<script>
import Navbar from './views/public/Navbar.vue'
import Footer from './views/public/Footer.vue'
import { EventSourcePolyfill } from 'event-source-polyfill'

export default {
    name: 'App',
    components: { Navbar, Footer },
    data() {
        return {
            event: {}
        }
    },
    computed: {
        jwt() {
            return this.$store.state.token
        }
    },
    watch: {
        jwt() {
            this.getEvent()
        }
    },
    created() {
        this.$store.actions.fetchConfig(this.getEvent())
        window.addEventListener('beforeunload', (e) => {
            this.$store.actions.saveConfig2Local()
        })
    },
    methods: {
        sleep(ms) {
            return new Promise((resolve) => setTimeout(resolve, ms))
        },
        getEvent() {
            let token = this.$store.state.token
            if (!token) return false
            if (!this.$store.state.isLogin || this.$store.state._sse) {
                return false
            }
            let es = new EventSourcePolyfill('/api/user/event', {
                headers: {
                    Authorization: `Bearer ${token}`
                }
            })
            this.$store.state._sse = es
            es.addEventListener('message', (event) => {
                try {
                    let data = JSON.parse(event.data)
                    this.handleEvent(data)
                } catch (e) {
                    console.error(e)
                }
            })
        },
        handleEvent(data) {
            if (!Array.isArray(data) || data.length === 0) {
                return false
            }
            data.forEach(async (d) => {
                let opt = d.opt
                if (!opt.msg || this.event[d.id]) {
                    return false
                }
                this.event[d.id] = true
                await this.sleep(1000)
                let mnOpt = {
                    message: opt.msg,
                    type: opt.theme || 'info',
                    dangerouslyUseHTMLString: opt.is_html,
                    duration: 0,
                    showClose: true,
                    onClose: () => {
                        this.deleteEvent(d.id)
                    }
                }
                if (opt.classify === 'message') {
                    this.$message(
                        Object.assign(
                            { customClass: 'el-message--slim' },
                            mnOpt
                        )
                    )
                } else if (opt.classify === 'notify') {
                    let ni = this.$notify(
                        Object.assign(
                            {
                                title: opt.title,
                                position: 'bottom-right',
                                onClick: () => {
                                    let jump = opt.notify_jump
                                    if (jump) {
                                        this.$router.push(jump)
                                        ni.close()
                                    }
                                }
                            },
                            mnOpt
                        )
                    )
                } else if (opt.classify === 'alert') {
                    this.$alert(opt.msg, opt.title || '提示', {
                        confirmButtonText: '确定',
                        type: opt.theme,
                        dangerouslyUseHTMLString: opt.is_html,
                        buttonSize: 'mini',
                        callback: (action) => {
                            console.log(action)
                            if (action === 'confirm') {
                                this.deleteEvent(d.id)
                            }
                        }
                    })
                }
            })
        },
        deleteEvent(id) {
            this.$http.delete(`/user/event/${id}`).then(() => {
                this.$message.success({
                    message: '事件已标识为已读',
                    customClass: 'el-message--slim'
                })
            })
        }
    }
}
</script>

<style>
#app {
    font-family: Avenir, Helvetica, Arial, sans-serif;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
    text-align: center;
    color: #2c3e50;
}
body > .el-container {
    margin-bottom: 40px;
}
.el-header,
.el-footer {
    line-height: 60px;
}
.el-message--slim {
    min-width: 150px;
}
.el-message-box--slim {
    width: 300px;
}
</style>
