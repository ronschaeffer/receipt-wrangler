import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:openapi/openapi.dart';
import 'package:receipt_wrangler_mobile/client/client.dart';
import 'package:receipt_wrangler_mobile/services/token_refresh_service.dart';

import '../helpers/auth_test_helpers.dart';

/// Cold-start / resume session-persistence regression tests.
///
/// These exercise the REAL [TokenRefreshService] against an [AuthApi] mock
/// that enforces the backend's *one-time-use* refresh-token semantics: once a
/// given refresh token has been presented to /token/, presenting it again is
/// rejected with 401 (the token was rotated/burned on first use).
///
/// This is the seam the existing suite does not cover: the existing
/// serialization tests only dedupe refreshes that overlap *in flight*. On
/// cold start, main.dart fires an un-awaited `refreshTokens()` AND the
/// GoRouter redirect calls `refreshTokens()`; the resume handler also calls
/// `refreshTokens(force: true)`. These can run BACK TO BACK (not overlapping),
/// so the second call re-sends a refresh token the first already rotated ->
/// 401 -> purgeTokens() -> user logged out.
class _OneTimeRefreshAuthApi extends Mock implements AuthApi {}

void main() {
  late MockAuthModel mockAuthModel;
  late MockGroupModel mockGroupModel;
  late MockOpenapi mockClient;
  late _OneTimeRefreshAuthApi mockAuthApi;
  late TokenRefreshService service;

  setUpAll(() {
    registerFallbackValue(FakeLogoutCommand());
  });

  setUp(() {
    mockAuthModel = MockAuthModel();
    mockGroupModel = MockGroupModel();
    mockClient = MockOpenapi();
    mockAuthApi = _OneTimeRefreshAuthApi();

    when(() => mockClient.getAuthApi()).thenReturn(mockAuthApi);
    when(() => mockClient.getUserApi()).thenReturn(MockUserApi());
    OpenApiClient.client = mockClient;

    service = TokenRefreshService();
    service.resetForTesting();
    service.initialize(
      authModel: mockAuthModel,
      groupModel: mockGroupModel,
      userModel: MockUserModel(),
      userPreferencesModel: MockUserPreferencesModel(),
      categoryModel: MockCategoryModel(),
      tagModel: MockTagModel(),
      systemSettingsModel: MockSystemSettingsModel(),
    );
    // Groups already populated so _loadAppDataIfNeeded() is a no-op and the
    // test isolates the token-refresh behaviour only.
    when(() => mockGroupModel.groups).thenReturn([MockGroup()]);
  });

  /// Wires the mock auth model + auth api to emulate persisted, rotating,
  /// one-time-use tokens. The "stored" tokens live in these local vars and are
  /// mutated by setTokens, exactly like FlutterSecureStorage would persist
  /// them across the sequential refresh calls within a single cold start.
  void wireRotatingStore({
    required String initialJwt,
    required String initialRefresh,
  }) {
    var storedJwt = initialJwt;
    var storedRefresh = initialRefresh;
    final consumedRefreshTokens = <String>{};
    var counter = 0;

    when(() => mockAuthModel.getJwt()).thenAnswer((_) async => storedJwt);
    when(() => mockAuthModel.getRefreshToken())
        .thenAnswer((_) async => storedRefresh);
    when(() => mockAuthModel.purgeTokens()).thenAnswer((_) async {
      storedJwt = '';
      storedRefresh = '';
    });
    when(() => mockAuthModel.setTokens(any(), any()))
        .thenAnswer((invocation) async {
      storedJwt = invocation.positionalArguments[0] as String;
      storedRefresh = invocation.positionalArguments[1] as String;
    });

    when(() => mockAuthApi.getNewRefreshToken(
        logoutCommand: any(named: 'logoutCommand'))).thenAnswer((invocation) async {
      final cmd =
          invocation.namedArguments[#logoutCommand] as LogoutCommand;
      final presented = cmd.refreshToken ?? '';

      // One-time-use: a refresh token already burned is rejected with 401.
      if (consumedRefreshTokens.contains(presented)) {
        throw DioException(
          requestOptions: RequestOptions(path: '/token/'),
          response: Response(
            statusCode: 401,
            requestOptions: RequestOptions(path: '/token/'),
          ),
        );
      }
      consumedRefreshTokens.add(presented);
      counter++;
      // Issue a fresh rotated pair.
      return createTokenRefreshResponse(validJwt, 'refresh-$counter');
    });
  }

  group('cold-start session persistence (one-time-use refresh tokens)', () {
    test(
        'SEQUENTIAL refresh after token rotation must NOT log the user out '
        '(cold-start expired-JWT path stays logged in)', () async {
      // Stored JWT is expired (app was closed long enough for the short-lived
      // access token to lapse) but the refresh token is still valid. This is
      // the everyday "reopen the app after a while" cold start.
      wireRotatingStore(
        initialJwt: expiredJwt,
        initialRefresh: validJwt,
      );

      // First refresh: main.dart's un-awaited _initFuture. Rotates the token.
      final first = await service.refreshTokens();
      expect(first, true, reason: 'first cold-start refresh should succeed');

      // Second refresh: the GoRouter redirect (unprotectedRouteRedirect),
      // running right AFTER the first completed (completer already cleared,
      // so no in-flight sharing). With the fix, the service should recognise
      // the freshly-rotated, still-valid JWT and NOT re-spend a dead token.
      final second = await service.refreshTokens();

      expect(second, true,
          reason:
              'second sequential refresh on a freshly-valid session must not '
              'fail — re-spending a rotated one-time token logs the user out');
      verifyNever(() => mockAuthModel.purgeTokens());
    });

    test(
        'repeated resume/timer refreshes on a healthy session must NOT log out',
        () async {
      // Valid JWT and valid refresh token — a healthy logged-in session.
      wireRotatingStore(
        initialJwt: validJwt,
        initialRefresh: validJwt,
      );

      // After the fix, _onResumed() and the 15-min timer both call the
      // NON-forced refreshTokens(). On a still-valid JWT this must be a no-op
      // (no token rotation), so firing it many times in a row never burns a
      // one-time refresh token and never purges the session.
      for (var i = 0; i < 5; i++) {
        final r = await service.refreshTokens();
        expect(r, true, reason: 'refresh #\$i on a valid session should hold');
      }
      verifyNever(() => mockAuthModel.purgeTokens());
      // A valid JWT means the refresh endpoint should never have been hit.
      verifyNever(() => mockAuthApi.getNewRefreshToken(
          logoutCommand: any(named: 'logoutCommand')));
    });

    test(
        'forced refresh (interceptor path) still rotates exactly once and holds',
        () async {
      // Interceptor forces a refresh after a real 401. Even though it forces,
      // a single forced rotation must succeed and must not double-spend.
      wireRotatingStore(
        initialJwt: validJwt,
        initialRefresh: validJwt,
      );

      final forced = await service.refreshTokens(force: true);
      expect(forced, true);
      verifyNever(() => mockAuthModel.purgeTokens());
    });
  });
}
