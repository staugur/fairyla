<template>
    <div
        class="back_to_top"
        v-show="showTop"
        @click="scrollToY(0, 1500, 'easeInOutQuint')"
    >
        <i class="saintic-icon saintic-icon-top"></i>
    </div>
</template>

<script>
window.requestAnimFrame = (function () {
    return (
        window.requestAnimationFrame ||
        window.webkitRequestAnimationFrame ||
        window.mozRequestAnimationFrame ||
        function (callback) {
            window.setTimeout(callback, 1000 / 60)
        }
    )
})()

export default {
    name: 'Backtop',
    data() {
        return {
            scrollTop: 0
        }
    },
    methods: {
        scrollToY(scrollTargetY, speed, easing) {
            // scrollTargetY: the target scrollY property of the window
            // speed: time in pixels per second
            // easing: easing equation to use

            let scrollY = window.scrollY || document.documentElement.scrollTop
            scrollTargetY = scrollTargetY || 0
            speed = speed || 2000
            easing = easing || 'easeOutSine'
            let currentTime = 0
            // min time .1, max time .8 seconds
            let time = Math.max(
                0.1,
                Math.min(Math.abs(scrollY - scrollTargetY) / speed, 0.8)
            )

            // easing equations from https://github.com/danro/easing-js/blob/master/easing.js
            let easingEquations = {
                easeOutSine: function (pos) {
                    return Math.sin(pos * (Math.PI / 2))
                },
                easeInOutSine: function (pos) {
                    return -0.5 * (Math.cos(Math.PI * pos) - 1)
                },
                easeInOutQuint: function (pos) {
                    if ((pos /= 0.5) < 1) {
                        return 0.5 * Math.pow(pos, 5)
                    }
                    return 0.5 * (Math.pow(pos - 2, 5) + 2)
                }
            }

            // add animation loop
            function tick() {
                currentTime += 1 / 60

                let p = currentTime / time
                let t = easingEquations[easing](p)

                if (p < 1) {
                    window.requestAnimFrame(tick)
                    window.scrollTo(0, scrollY + (scrollTargetY - scrollY) * t)
                } else {
                    window.scrollTo(0, scrollTargetY)
                }
            }

            // call it once to get started
            tick()
        },
        getScrollTop() {
            this.scrollTop =
                window.pageYOffset ||
                document.documentElement.scrollTop ||
                document.body.scrollTop
        }
    },
    computed: {
        showTop: function () {
            return this.scrollTop > 500
        }
    },
    mounted() {
        window.addEventListener('scroll', this.getScrollTop)
    }
}
</script>

<style scoped>
.back_to_top {
    position: fixed;
    z-index: 99999;
    bottom: 2.5rem;
    right: 1.8rem;
    background-color: #fff;
    color: #409eff;
    border-radius: 50%;
    width: 40px;
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 20px;
    box-shadow: 0 0 6px rgb(0 0 0 / 12%);
    cursor: pointer;
}
</style>
