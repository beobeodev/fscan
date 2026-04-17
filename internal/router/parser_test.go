package router

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile writes content to projectRoot/rel creating parent dirs as needed.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func findRoute(defs []*RouteDef, label string) *RouteDef {
	for _, d := range defs {
		if d.Label == label {
			return d
		}
	}
	return nil
}

func TestParse_AutoRouteFromGrDart(t *testing.T) {
	root := t.TempDir()
	// Widget class annotated with @RoutePage
	writeFile(t, root, "lib/home_page.dart", `import 'package:auto_route/auto_route.dart';
@RoutePage()
class HomePage extends StatelessWidget {
  const HomePage({super.key});
}
`)
	// Generated .gr.dart maps HomePage → HomeRoute (Page suffix stripped)
	writeFile(t, root, "lib/router.gr.dart", `// GENERATED CODE
/// generated route for
/// [_i1.HomePage]
class HomeRoute extends PageRouteInfo {
  const HomeRoute();
}
`)

	defs, err := Parse(root)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := findRoute(defs, "HomeRoute")
	if r == nil {
		t.Fatalf("expected HomeRoute from .gr.dart mapping, got: %+v", defs)
	}
	if r.Kind != "auto_route" {
		t.Errorf("kind = %s, want auto_route", r.Kind)
	}
}

func TestParse_AutoRouteFallbackSuffix(t *testing.T) {
	// No .gr.dart — parser should fall back to <ClassName>Route.
	root := t.TempDir()
	writeFile(t, root, "lib/settings.dart", `@RoutePage()
class SettingsWidget extends StatelessWidget {}
`)
	defs, _ := Parse(root)
	if findRoute(defs, "SettingsWidgetRoute") == nil {
		t.Errorf("expected fallback SettingsWidgetRoute, got: %+v", defs)
	}
}

func TestParse_AutoRouteInitialFlag(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "lib/app_router.dart", `final routes = [
  AutoRoute(page: HomeRoute.page, initial: true),
  AutoRoute(page: ProfileRoute.page),
];
@RoutePage()
class HomePage extends StatelessWidget {}
@RoutePage()
class ProfilePage extends StatelessWidget {}
`)
	writeFile(t, root, "lib/app.gr.dart", `/// [_i1.HomePage]
class HomeRoute extends PageRouteInfo {}
/// [_i1.ProfilePage]
class ProfileRoute extends PageRouteInfo {}
`)
	defs, _ := Parse(root)
	home := findRoute(defs, "HomeRoute")
	prof := findRoute(defs, "ProfileRoute")
	if home == nil || !home.Initial {
		t.Errorf("HomeRoute should be initial, got %+v", home)
	}
	if prof == nil || prof.Initial {
		t.Errorf("ProfileRoute should NOT be initial, got %+v", prof)
	}
}

func TestParse_GoRouterPathAndName(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "lib/go.dart", `final router = GoRouter(
  initialLocation: '/home',
  routes: [
    GoRoute(
      path: '/home',
      name: 'home',
      builder: (c, s) => const HomePage(),
    ),
    GoRoute(
      path: '/profile',
      name: 'profile',
      builder: (c, s) => const ProfilePage(),
    ),
  ],
);
`)
	defs, _ := Parse(root)

	home := findRoute(defs, "/home")
	if home == nil {
		t.Fatalf("expected /home GoRoute, got %+v", defs)
	}
	if home.Kind != "go_router" {
		t.Errorf("kind = %s, want go_router", home.Kind)
	}
	if !home.Initial {
		t.Errorf("/home should be initial (matches initialLocation)")
	}
	// Both path and name should appear as identifiers.
	hasName := false
	for _, id := range home.Identifiers {
		if id == "home" {
			hasName = true
		}
	}
	if !hasName {
		t.Errorf("expected 'home' in identifiers, got %v", home.Identifiers)
	}

	prof := findRoute(defs, "/profile")
	if prof == nil || prof.Initial {
		t.Errorf("/profile should exist and not be initial, got %+v", prof)
	}
}

func TestParse_NavigatorNamedRoutes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "lib/nav.dart", `final routes = {
  '/': (ctx) => const HomeScreen(),
  '/settings': (ctx) => const SettingsScreen(),
  'notARoute': 'plain value',
};
`)
	defs, _ := Parse(root)
	if r := findRoute(defs, "/"); r == nil || !r.Initial || r.Kind != "navigator" {
		t.Errorf("expected / as initial navigator route, got %+v", r)
	}
	if r := findRoute(defs, "/settings"); r == nil || r.Initial {
		t.Errorf("expected /settings non-initial navigator route, got %+v", r)
	}
	if findRoute(defs, "notARoute") != nil {
		t.Error("non-path map keys should not be captured as routes")
	}
}

func TestParse_SkipsGeneratedFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "lib/a.gr.dart", `class FooRoute extends PageRouteInfo {}`)
	writeFile(t, root, "lib/a.g.dart", `class BarRoute {}`)
	writeFile(t, root, "lib/a.freezed.dart", `class BazRoute {}`)

	defs, _ := Parse(root)
	for _, d := range defs {
		if d.File == "lib/a.gr.dart" || d.File == "lib/a.g.dart" || d.File == "lib/a.freezed.dart" {
			t.Errorf("generated file parsed: %+v", d)
		}
	}
}
