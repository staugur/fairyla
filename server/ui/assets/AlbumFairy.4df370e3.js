import{i as a,f as s}from"./index.39d10135.js";import{a0 as e,a1 as r,X as i,V as t,c as l,a as c,k as n,F as u,C as o,K as d,i as m}from"./vendor.884d25e1.js";e("data-v-51c87a6e");const f={class:"container"},p={class:"wrapper"},b={class:"entry"},y={class:"post-head"},h={class:"post-info"},v=c("br",null,null,-1),g={class:"bdshare"},w={class:"info"},A={key:0,class:"category"},k=c("i",{class:"saintic-icon saintic-icon-tags"},null,-1),x={class:"date"},F=c("i",{class:"saintic-icon saintic-icon-time"},null,-1);r();const j={expose:[],props:{album:{type:Object,required:!0},fairies:{type:Array,required:!0,validator:s=>{if(!Array.isArray(s))return!1;for(let e of s){if(!a(e))return!1;if(!e.hasOwnProperty("src"))return!1}return!0}},urls:Array},setup:a=>(s,e)=>{const r=i("el-image");return t(),l("section",f,[c("section",p,[c("article",b,[c("header",y,[c("h2",null,n(a.album.name),1)]),c("div",h,"    所属："+n(a.album.owner),1),(t(!0),l(u,null,o(a.fairies,(s=>(t(),l("div",{style:{"text-align":"center"},key:s.id},[c(r,{title:s.desc,src:s.src,lazy:!0,"preview-src-list":a.urls,"hide-on-click-modal":!0},null,8,["title","src","preview-src-list"])])))),128)),v,c("section",g,[c("div",w,[a.album.label?(t(),l("span",A,[k,d(" "+n(a.album.label),1)])):m("",!0),c("span",x,[F,d(" "+n(a.album.cdate||a.album.ctime),1)])])])])])])},__scopeId:"data-v-51c87a6e"},$={Name:"AlbumFairy",components:{Fairy:j},data:()=>({album:{},fairies:[],urls:[]}),created(){let a=this.$route.params.name;this.$http.get(`/user/album/${a}?fairy=true`).then((a=>{this.album=a.data,this.album.cdate=s(a.data.ctime),this.fairies=a.data.fairy;for(let s of this.fairies)this.urls.push(s.src)}))}};$.render=function(a,s,e,r,c,n){const u=i("Fairy");return t(),l(u,{album:c.album,fairies:c.fairies,urls:c.urls},null,8,["album","fairies","urls"])};export default $;
