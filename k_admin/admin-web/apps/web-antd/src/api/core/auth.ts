import { baseRequestClient, requestClient } from '#/api/request';

export namespace AuthApi {
  /** 登录接口参数 */
  export interface LoginParams {
    password?: string;
    username?: string;
    captchaAnswer?: string;
    captchaId?: string;
  }

  /** 登录接口返回值 */
  export interface LoginResult {
    accessToken: string;
    expiresAt?: number;
    refreshToken?: string;
    token?: string;
    tokenType?: string;
  }

  export interface CaptchaChallenge {
    enabled: boolean;
    expiresIn?: number;
    id?: string;
    image?: string;
  }

  export interface LogoutParams {
    accessToken?: null | string;
    refreshToken?: null | string;
  }

  export interface RefreshTokenResponse {
    code: number;
    data: LoginResult;
    message: string;
    msg?: string;
  }
}

export async function getCaptchaApi() {
  const response = await baseRequestClient.get<any>('/auth/captcha');
  return (response.data?.data ?? response.data) as AuthApi.CaptchaChallenge;
}

/**
 * 登录
 */
export async function loginApi(data: AuthApi.LoginParams) {
  return requestClient.post<AuthApi.LoginResult>('/auth/login', data, {
    skipAuthRefresh: true,
  });
}

/**
 * 刷新accessToken
 */
export async function refreshTokenApi(refreshToken: string) {
  const response = await baseRequestClient.post<any>('/auth/refresh', {
    refreshToken,
  });
  return response.data?.data as AuthApi.LoginResult;
}

/**
 * 退出登录
 */
export async function logoutApi(params: AuthApi.LogoutParams = {}) {
  return baseRequestClient.post(
    '/auth/logout',
    {
      refreshToken: params.refreshToken,
    },
    {
      headers: params.accessToken
        ? { Authorization: `Bearer ${params.accessToken}` }
        : undefined,
    },
  );
}

/**
 * 获取用户权限码
 */
export async function getAccessCodesApi() {
  return requestClient.get<string[]>('/auth/codes');
}
