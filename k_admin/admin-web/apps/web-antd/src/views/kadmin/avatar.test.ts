import { describe, expect, it } from 'vitest';

import { avatarLabel } from './avatar';

describe('avatarLabel', () => {
  it('collapses bundled default avatar paths written by the desktop client', () => {
    expect(
      avatarLabel(
        'file:///D:/Max/pezmax/resources/app.asar/out/renderer/static/jpg/default_avatar-BE-pmgET.jpg',
      ),
    ).toBe('default_avatar');
    expect(avatarLabel('/src/renderer/assets/images/default_avatar.jpg')).toBe(
      'default_avatar',
    );
    expect(
      avatarLabel(String.raw`D:\pezmax\static\jpg\default_avatar-BE.jpg`),
    ).toBe('default_avatar');
    expect(avatarLabel('default.png')).toBe('default_avatar');
    expect(avatarLabel('default_avatar.jpg?v=3')).toBe('default_avatar');
  });

  it('keeps uploaded avatars and empty values distinguishable', () => {
    expect(avatarLabel('https://cdn.example.com/u/7/photo.jpg?v=2')).toBe(
      'https://cdn.example.com/u/7/photo.jpg?v=2',
    );
    expect(avatarLabel('/api/files/12/content')).toBe('/api/files/12/content');
    expect(avatarLabel('file:///D:/Max/pezmax/upload/photo.jpg')).toBe(
      'file:///D:/Max/pezmax/upload/photo.jpg',
    );
    expect(avatarLabel(undefined)).toBe('-');
    expect(avatarLabel('   ')).toBe('-');
    expect(avatarLabel('null')).toBe('-');
  });
});
