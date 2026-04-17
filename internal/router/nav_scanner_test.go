package router

import (
	"testing"
)

func TestCollectNavReferences_GoRouterCalls(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "lib/callsite.dart", `void navigate(BuildContext ctx) {
  context.go('/home');
  context.push('/profile');
  context.goNamed('settings');
  context.pushNamed('cart');
  GoRouter.of(ctx).pushReplacement('/login');
}
`)

	refs := CollectNavReferences(root, nil)
	for _, want := range []string{"/home", "/profile", "settings", "cart", "/login"} {
		if !refs[want] {
			t.Errorf("expected ref %q in %v", want, refs)
		}
	}
}

func TestCollectNavReferences_NavigatorCalls(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "lib/callsite.dart", `void go(BuildContext ctx) {
  Navigator.pushNamed(ctx, '/detail');
  Navigator.pushReplacementNamed(ctx, '/main');
  Navigator.popAndPushNamed(ctx, '/back');
}
`)
	refs := CollectNavReferences(root, nil)
	for _, want := range []string{"/detail", "/main", "/back"} {
		if !refs[want] {
			t.Errorf("expected ref %q in %v", want, refs)
		}
	}
}

func TestCollectNavReferences_AutoRouteConstructors(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "lib/use.dart", `void go() {
  context.router.push(const HomeRoute());
  context.router.push(ProfileRoute(userId: '1'));
}
`)
	refs := CollectNavReferences(root, []string{"HomeRoute", "ProfileRoute", "UnusedRoute"})
	if !refs["HomeRoute"] {
		t.Errorf("expected HomeRoute ref, got %v", refs)
	}
	if !refs["ProfileRoute"] {
		t.Errorf("expected ProfileRoute ref, got %v", refs)
	}
	if refs["UnusedRoute"] {
		t.Errorf("UnusedRoute should not be referenced, got %v", refs)
	}
}

func TestCollectNavReferences_EmptyWhenNoCalls(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "lib/nocalls.dart", `class Foo {}`)
	refs := CollectNavReferences(root, []string{"HomeRoute"})
	if len(refs) != 0 {
		t.Errorf("expected no refs, got %v", refs)
	}
}
