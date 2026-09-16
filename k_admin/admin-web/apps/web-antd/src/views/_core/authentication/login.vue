<script lang="ts" setup>
import type { AuthApi } from '#/api';
import type { VbenFormSchema } from '@vben/common-ui';

import { computed, h, onMounted, ref } from 'vue';

import { AuthenticationLogin, z } from '@vben/common-ui';
import { $t } from '@vben/locales';

import { getCaptchaApi } from '#/api';
import { useAuthStore } from '#/store';

defineOptions({ name: 'Login' });

const authStore = useAuthStore();
const loginRef = ref<InstanceType<typeof AuthenticationLogin>>();
const captcha = ref<AuthApi.CaptchaChallenge>({ enabled: false });
const captchaLoading = ref(false);

const formSchema = computed((): VbenFormSchema[] => {
  const schemas: VbenFormSchema[] = [
    {
      component: 'VbenInput',
      componentProps: {
        placeholder: $t('authentication.usernameTip'),
      },
      defaultValue: 'admin',
      fieldName: 'username',
      label: $t('authentication.username'),
      rules: z.string().min(1, { message: $t('authentication.usernameTip') }),
    },
    {
      component: 'VbenInputPassword',
      componentProps: {
        placeholder: $t('authentication.password'),
      },
      defaultValue: 'admin',
      fieldName: 'password',
      label: $t('authentication.password'),
      rules: z.string().min(1, { message: $t('authentication.passwordTip') }),
    },
  ];

  if (captcha.value.enabled) {
    schemas.push({
      component: 'VbenInput',
      componentProps: {
        autocomplete: 'off',
        maxlength: 6,
        placeholder: '请输入图片中的 6 位数字',
      },
      fieldName: 'captchaAnswer',
      label: '验证码',
      rules: z.string().regex(/^\d{6}$/, { message: '请输入 6 位数字验证码' }),
      suffix: () =>
        h(
          'button',
          {
            'aria-label': '刷新验证码',
            class:
              'flex h-10 w-[122px] shrink-0 items-center justify-center overflow-hidden rounded-md border bg-muted text-xs text-muted-foreground disabled:cursor-wait',
            disabled: captchaLoading.value,
            title: '点击刷新验证码',
            type: 'button',
            onClick: refreshCaptcha,
          },
          captcha.value.image
            ? h('img', {
                alt: '图片验证码，点击刷新',
                class: 'h-full w-full object-fill',
                draggable: false,
                src: captcha.value.image,
              })
            : captchaLoading.value
              ? '加载中…'
              : '点击重新加载',
        ),
    });
  }

  return schemas;
});

async function refreshCaptcha() {
  if (captchaLoading.value) return;
  captchaLoading.value = true;
  try {
    captcha.value = await getCaptchaApi();
  } catch {
    captcha.value = { enabled: true };
  } finally {
    captchaLoading.value = false;
    loginRef.value?.getFormApi().setFieldValue('captchaAnswer', '');
  }
}

async function handleSubmit(values: Record<string, unknown>) {
  try {
    await authStore.authLogin({
      ...values,
      captchaId: captcha.value.id,
    });
  } catch (error) {
    if (captcha.value.enabled) await refreshCaptcha();
    throw error;
  }
}

onMounted(refreshCaptcha);
</script>

<template>
  <AuthenticationLogin
    ref="loginRef"
    :form-schema="formSchema"
    :loading="authStore.loginLoading"
    @submit="handleSubmit"
  />
</template>
